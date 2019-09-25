package extendedjob

import (
	"context"

	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// NewJobReconciler returns a new Reconciler
func NewJobReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) (reconcile.Reconciler, error) {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileJob{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		versionedSecretStore: versionedSecretStore,
	}, nil
}

// ReconcileJob reconciles an Job object
type ReconcileJob struct {
	ctx                  context.Context
	client               client.Client
	scheme               *runtime.Scheme
	config               *config.Config
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// Reconcile reads that state of the cluster for a Job object that is owned by an ExtendedJob and
// makes changes based on the state read and what is in the ExtendedJob.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &batchv1.Job{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling job output '%s' in the ExtendedJob context", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Info(ctx, "Skip reconcile: Job not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, err
	}

	// Get the job's extended job parent
	parentName := ""
	for _, owner := range instance.GetOwnerReferences() {
		if *owner.Controller {
			parentName = owner.Name
		}
	}
	if parentName == "" {
		err = ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "Could not find parent ExtendedJob for Job '%s'", request.NamespacedName)
		return reconcile.Result{}, err
	}

	ej := ejv1.ExtendedJob{}
	err = r.client.Get(ctx, types.NamespacedName{Name: parentName, Namespace: instance.GetNamespace()}, &ej)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "getting parent ExtendedJob in Job Reconciler for job %s", instance.GetName())
	}

	// Delete Job if it succeeded
	if instance.Status.Succeeded == 1 {
		ctxlog.WithEvent(&ej, "DeletingJob").Infof(ctx, "Deleting succeeded job '%s'", instance.Name)
		err = r.client.Delete(ctx, instance)
		if err != nil {
			ctxlog.WithEvent(instance, "DeleteError").Errorf(ctx, "Cannot delete succeeded job: '%s'", err)
		}

		if d, ok := instance.Spec.Template.Labels["delete"]; ok {
			if d == "pod" {
				pod, err := r.jobPod(ctx, instance.Name, instance.GetNamespace())
				if err != nil {
					ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "Cannot find job's pod: '%s'", err)
					return reconcile.Result{}, nil
				}
				ctxlog.WithEvent(&ej, "DeletingJobsPod").Infof(ctx, "Deleting succeeded job's pod '%s'", pod.Name)
				err = r.client.Delete(ctx, pod)
				if err != nil {
					ctxlog.WithEvent(instance, "DeleteError").Errorf(ctx, "Cannot delete succeeded job's pod: '%s'", err)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

// jobPod gets the job's pod. Only single-pod jobs are supported when persisting the output, so we just get the first one.
func (r *ReconcileJob) jobPod(ctx context.Context, name string, namespace string) (*corev1.Pod, error) {
	list := &corev1.PodList{}
	err := r.client.List(
		ctx,
		list,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{"job-name": name}),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "Listing job's %s pods failed.", name)
	}
	if len(list.Items) == 0 {
		return nil, errors.Errorf("Job %s does not own any pods?", name)
	}

	// If there is only one jobpod, then return index 0 pod
	latestPod := list.Items[0]
	if len(list.Items) > 1 {
		// If there are more than one jobpods, then return the latest
		// created jobpod. There will be multiple jobpods when jobpods
		// fail. Kubernetes job creates new jobpods if the jobpod
		// created previously fails
		latestTimeStamp := list.Items[0].GetCreationTimestamp().UTC()
		for podIndex, pod := range list.Items {
			if latestTimeStamp.Before(pod.GetCreationTimestamp().UTC()) {
				latestTimeStamp = pod.GetCreationTimestamp().UTC()
				latestPod = list.Items[podIndex]
			}
		}
	}

	ctxlog.Infof(ctx, "Considering job pod %s for persisting output", latestPod.GetName())
	return &latestPod, nil
}
