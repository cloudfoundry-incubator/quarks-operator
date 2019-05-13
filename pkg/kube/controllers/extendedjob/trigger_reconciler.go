package extendedjob

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

var _ reconcile.Reconciler = &TriggerReconciler{}

type setOwnerReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewTriggerReconciler returns a new reconcile to start jobs triggered by pods
func NewTriggerReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	query Query,
	f setOwnerReferenceFunc,
) reconcile.Reconciler {
	return &TriggerReconciler{
		ctx:               ctx,
		client:            mgr.GetClient(),
		config:            config,
		query:             query,
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
	}
}

// TriggerReconciler implements the Reconciler interface
type TriggerReconciler struct {
	ctx               context.Context
	client            client.Client
	config            *config.Config
	query             Query
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
}

// Reconcile creates jobs for extended jobs which match the request's pod.
// When there are multiple extendedjobs, multiple jobs can run for the same
// pod.
func (r *TriggerReconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	podName := request.NamespacedName.Name

	pod := &corev1.Pod{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	err = r.client.Get(ctx, request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, pod is probably deleted
			ctxlog.Debugf(ctx, "Failed to find pod, not retrying: %s", err)
			err = nil
			return
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get the pod: %s", err)
		return
	}

	podState := InferPodState(*pod)
	if podState == ejv1.PodStateUnknown {
		ctxlog.Debugf(ctx,
			"Failed to determine state %s: %#v",
			PodStatusString(*pod),
			pod.Status,
		)
		return
	}

	eJobs := &ejv1.ExtendedJobList{}
	err = r.client.List(ctx, &client.ListOptions{}, eJobs)
	if err != nil {
		ctxlog.Infof(ctx, "Failed to query extended jobs: %s", err)
		return
	}

	if len(eJobs.Items) < 1 {
		return
	}

	podEvent := fmt.Sprintf("%s/%s", podName, podState)
	ctxlog.Debugf(ctx, "Considering %d extended jobs for pod %s", len(eJobs.Items), podEvent)

	for _, eJob := range eJobs.Items {
		if r.query.MatchState(eJob, podState) && r.query.Match(eJob, *pod) {
			err := r.createJob(ctx, eJob, podName, string(pod.UID))
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					ctxlog.Debugf(ctx, "Skip '%s' triggered by pod %s: already running", eJob.Name, podEvent)
				} else {
					ctxlog.WithEvent(&eJob, "CreateJob").Infof(ctx, "Failed to create job for '%s' via pod %s: %s", eJob.Name, podEvent, err)
				}
				continue
			}
			ctxlog.WithEvent(&eJob, "CreateJob").Infof(ctx, "Created job for '%s' via pod %s", eJob.Name, podEvent)
		}
	}
	return
}

func (r *TriggerReconciler) createJob(ctx context.Context, eJob ejv1.ExtendedJob, podName string, podUID string) error {
	template := eJob.Spec.Template.DeepCopy()

	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels[ejv1.LabelEJobName] = eJob.Name
	template.Labels[ejv1.LabelTriggeringPod] = podUID

	name, err := names.JobName(eJob.Name, podName)
	if err != nil {
		return errors.Wrapf(err, "could not generate job name for eJob '%s'", eJob.Name)
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: eJob.Namespace,
			Labels:    map[string]string{ejv1.LabelExtendedJob: "true"},
		},
		Spec: batchv1.JobSpec{Template: *template},
	}

	err = r.setOwnerReference(&eJob, job, r.scheme)
	if err != nil {
		ctxlog.WithEvent(&eJob, "SetOwnerReferenceError").Errorf(ctx, "failed to set owner reference on job for '%s' via pod %s: %s", eJob.Name, podName, err)
		return err
	}

	err = r.client.Create(ctx, job)
	if err != nil {
		return err
	}

	return nil
}
