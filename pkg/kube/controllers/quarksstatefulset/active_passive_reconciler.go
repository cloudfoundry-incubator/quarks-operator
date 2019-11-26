package quarksstatefulset

import (
	"context"
	"os"
	"path/filepath"
	"time"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	podutil "code.cloudfoundry.org/quarks-utils/pkg/pod"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewActivePassiveReconciler returns a new reconcile.Reconciler for the active/passive controller
func NewActivePassiveReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, kclient kubernetes.Interface) reconcile.Reconciler {
	return &ReconcileStatefulSetActivePassive{
		ctx:        ctx,
		config:     config,
		client:     mgr.GetClient(),
		kclient:    kclient,
		scheme:     mgr.GetScheme(),
		restConfig: mgr.GetConfig(),
	}
}

// ReconcileStatefulSetActivePassive reconciles an QuarksStatefulSet object when references change
type ReconcileStatefulSetActivePassive struct {
	ctx        context.Context
	client     crc.Client
	kclient    kubernetes.Interface
	scheme     *runtime.Scheme
	config     *config.Config
	restConfig *restclient.Config
}

// Reconcile reads the state of the cluster for a QuarksStatefulSet object
// and makes changes based on the state read and what is in the QuarksStatefulSet.Spec
// Note:
// The Reconcile Loop will always requeue the request stop before under completition. For this specific
// loop, the requeue will happen after the ActivePassiveProbe PeriodSeconds is reached.
func (r *ReconcileStatefulSetActivePassive) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	qSts := &qstsv1a1.QuarksStatefulSet{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling for active/passive QuarksStatefulSet", request.NamespacedName)

	if err := r.client.Get(ctx, request.NamespacedName, qSts); err != nil {
		if apierrors.IsNotFound(err) {
			// Reconcile successful - don't requeue
			ctxlog.Infof(ctx, "Failed to find quarks statefulset '%s', not retrying: %s", request.NamespacedName, err)
			return reconcile.Result{}, nil
		}
		// Reconcile failed due to error - requeue
		ctxlog.Errorf(ctx, "Failed to get quarks statefulset '%s': %s", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	statefulSets, _, err := GetMaxStatefulSetVersion(ctx, r.client, qSts)
	if err != nil {
		// Reconcile failed due to error - requeue
		return reconcile.Result{}, errors.Wrapf(err, "couldn't list StatefulSets for active/passive reconciliation")
	}

	ownedPods, err := r.getStsPodList(ctx, statefulSets)
	if err != nil {
		// Reconcile failed due to error - requeue
		return reconcile.Result{}, errors.Wrapf(err, "couldn't retrieve pod items from sts: %s", qSts.Name)
	}

	// retrieves the ActivePassiveProbe children key,
	// this is the container name in where the ActivePassiveProbe
	// cmd, needs to be executed
	containerName, err := getProbeContainerName(qSts.Spec.ActivePassiveProbe)
	if err != nil {
		// Reconcile failed due to error - requeue
		return reconcile.Result{}, errors.Wrapf(err, "None container name found in probe for %s QuarksStatefulSet", qSts.Name)
	}

	err = r.markActiveContainers(ctx, containerName, ownedPods, qSts)
	if err != nil {
		// Reconcile failed due to error - requeue
		return reconcile.Result{}, err
	}

	periodSeconds := time.Second * time.Duration(qSts.Spec.ActivePassiveProbe[containerName].PeriodSeconds)
	if periodSeconds == (time.Second * time.Duration(0)) {
		ctxlog.WithEvent(qSts, "active-passive").Debugf(ctx, "periodSeconds probe was not specified, going to default to 30 secs")
		periodSeconds = time.Second * 30
	}

	// Reconcile for any reason than error after the ActivePassiveProbe PeriodSeconds
	return reconcile.Result{RequeueAfter: periodSeconds}, nil
}

func (r *ReconcileStatefulSetActivePassive) markActiveContainers(ctx context.Context, container string, pods *corev1.PodList, qSts *qstsv1a1.QuarksStatefulSet) (err error) {

	probeCmd := qSts.Spec.ActivePassiveProbe[container].Exec.Command

	for _, pod := range pods.Items {
		ctxlog.WithEvent(qSts, "active-passive").Debugf(ctx, "validating probe in pod: %s", pod.Name)
		if err := r.execContainerCmd(&pod, container, probeCmd); err != nil {
			ctxlog.WithEvent(qSts, "active-passive").Debugf(
				ctx,
				"failed to execute active/passive probe: %s",
				err,
			)
			// mark as passive
			err := r.deleteActiveLabel(ctx, &pod, qSts)
			if err != nil {
				return errors.Wrapf(err, "couldn't remove label from active pod %s", pod.Name)
			}
		} else {
			if podutil.IsPodReady(&pod) {
				// mark as active
				err := r.addActiveLabel(ctx, &pod, qSts)
				if err != nil {
					return errors.Wrapf(err, "couldn't label pod %s as active", pod.Name)
				}
			}
		}
	}
	return nil
}

func (r *ReconcileStatefulSetActivePassive) addActiveLabel(ctx context.Context, p *corev1.Pod, qSts *qstsv1a1.QuarksStatefulSet) error {
	podLabels := p.GetLabels()
	if podLabels == nil {
		podLabels = map[string]string{}
	}

	if _, found := podLabels[qstsv1a1.LabelActiveContainer]; found {
		return nil
	}

	podLabels[qstsv1a1.LabelActiveContainer] = "active"

	return r.updatePodLabels(ctx, p, qSts, "active")
}

func (r *ReconcileStatefulSetActivePassive) deleteActiveLabel(ctx context.Context, p *corev1.Pod, qSts *qstsv1a1.QuarksStatefulSet) error {
	podLabels := p.GetLabels()

	if _, found := podLabels[qstsv1a1.LabelActiveContainer]; !found {
		return nil
	}
	delete(podLabels, qstsv1a1.LabelActiveContainer)

	return r.updatePodLabels(ctx, p, qSts, "passive")
}

func (r *ReconcileStatefulSetActivePassive) updatePodLabels(ctx context.Context, p *corev1.Pod, qSts *qstsv1a1.QuarksStatefulSet, mode string) error {
	err := r.client.Update(r.ctx, p)
	if err != nil {
		return err
	}

	ctxlog.WithEvent(qSts, "active-passive").Debugf(
		ctx,
		"pod %s promoted to %s",
		p.Name,
		mode,
	)
	return nil
}

func (r *ReconcileStatefulSetActivePassive) execContainerCmd(pod *corev1.Pod, container string, command []string) error {
	req := r.kclient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    false,
		}, clientscheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(r.restConfig, "POST", req.URL())
	if err != nil {
		return errors.New("failed to initialize remote command executor")
	}
	if err = executor.Stream(remotecommand.StreamOptions{Stdin: os.Stdin, Stdout: os.Stdout, Tty: false}); err != nil {
		return errors.Wrapf(err, "failed executing command in pod: %s, container: %s in namespace: %s",
			pod.Name,
			container,
			pod.Namespace,
		)
	}
	ctxlog.Info(r.ctx, "Succesfully exec cmd in container: ", container, ", inside pod: ", pod.Name)

	return nil
}

func (r *ReconcileStatefulSetActivePassive) getStsPodList(ctx context.Context, desiredSts *appsv1.StatefulSet) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := r.client.List(ctx,
		podList,
		crc.InNamespace(desiredSts.Namespace),
	)
	if err != nil {
		return nil, err
	}
	return podList, nil
}

func getProbeContainerName(p map[string]*corev1.Probe) (string, error) {
	for key := range p {
		return key, nil
	}
	return "", errors.New("failed to find a container key in the active/passive probe in the current QuarksStatefulSet")
}

// KubeConfig returns a kube config for this environment
func KubeConfig() (*rest.Config, error) {
	location := os.Getenv("KUBECONFIG")
	if location == "" {
		location = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", location)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}
