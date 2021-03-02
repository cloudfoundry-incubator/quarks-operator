package quarksrestart

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
)

// RestartKey has the timestamp of the last restart triggered by this reconciler
var RestartKey = fmt.Sprintf("%s/restart", apis.GroupName)

// NewRestartReconciler returns a new reconciler to restart deployments & statefulsets
func NewRestartReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRestart{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
	}
}

// ReconcileRestart contains necessary state for the reconcile
type ReconcileRestart struct {
	ctx    context.Context
	client client.Client
	config *config.Config
}

// Reconcile adds an annotation to deployments, statefulsets & jobs which own the pod
// whose referred secret has changed
func (r *ReconcileRestart) Reconcile(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Info(ctx, "Reconciling pod ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Return and don't requeue
			log.Debug(ctx, "Skip pod reconcile: pod not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if meltdown.NewAnnotationWindow(r.config.MeltdownDuration, pod.ObjectMeta.Annotations).Contains(time.Now()) {
		log.WithEvent(pod, "Meltdown").Debugf(ctx, "Resource '%s/%s' is in meltdown, requeue reconcile after %s", pod.Namespace, pod.Name, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	}

	// find owners and touch them
	for _, or := range pod.GetOwnerReferences() {
		if or.Kind == "StatefulSet" {
			err := r.touchStatefulSet(ctx, request.Namespace, or.Name)
			if err != nil {
				log.Debugf(ctx, "Skip pod reconcile: %s", err)
				return reconcile.Result{}, nil
			}
		} else if or.Kind == "ReplicaSet" {
			err := r.touchDeployment(ctx, request.Namespace, or.Name)
			if err != nil {
				log.Debugf(ctx, "Skip pod reconcile: %s", err)
				return reconcile.Result{}, nil
			}
		}
	}

	meltdown.SetLastReconcile(&pod.ObjectMeta, time.Now())
	err = r.client.Update(ctx, pod)
	if err != nil {
		log.WithEvent(pod, "UpdateError").Errorf(ctx, "Failed to update reconcile timestamp on restart annotated pod '%s/%s' (%v): %s", pod.Namespace, pod.Name, pod.ResourceVersion, err)
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileRestart) touchStatefulSet(ctx context.Context, namespace string, name string) error {
	sts := &appsv1.StatefulSet{}
	err := r.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, sts)
	if err != nil {
		return err
	}

	// Sts part of QuarksStatefulset have their own logic of updation
	for _, or := range sts.GetOwnerReferences() {
		if or.Kind == "QuarksStatefulSet" {
			return nil
		}
	}

	sts.Spec.Template.SetAnnotations(
		labels.Merge(sts.Spec.Template.GetAnnotations(), restartAnnotation()),
	)
	return r.client.Update(ctx, sts)
}

func (r *ReconcileRestart) touchDeployment(ctx context.Context, namespace string, name string) error {
	rs := &appsv1.ReplicaSet{}
	err := r.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, rs)
	if err != nil {
		return err
	}

	d, err := r.findDeployment(ctx, *rs)
	if err != nil {
		return err
	}

	d.Spec.Template.SetAnnotations(
		labels.Merge(d.Spec.Template.GetAnnotations(), restartAnnotation()),
	)
	return r.client.Update(ctx, d)
}

func (r *ReconcileRestart) findDeployment(ctx context.Context, rs appsv1.ReplicaSet) (*appsv1.Deployment, error) {
	for _, or := range rs.GetOwnerReferences() {
		if or.Kind == "Deployment" {
			d := &appsv1.Deployment{}
			err := r.client.Get(ctx, types.NamespacedName{
				Namespace: rs.GetNamespace(),
				Name:      or.Name,
			}, d)
			return d, err
		}
	}
	return nil, fmt.Errorf("deployment for replica set '%s/%s' was not found", rs.Namespace, rs.Name)
}

func restartAnnotation() map[string]string {
	return map[string]string{RestartKey: strconv.FormatInt(time.Now().Unix(), 10)}
}
