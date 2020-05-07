package quarksstatefulset

import (
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qstsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// NewQuarksStatefulSetStatusReconciler returns a new reconcile.Reconciler for QuarksStatefulSets Status
func NewQuarksStatefulSetStatusReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileQuarksStatefulSetStatus{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// ReconcileQuarksStatefulSetStatus reconciles an QuarksStatefulSet object for its status
type ReconcileQuarksStatefulSetStatus struct {
	ctx    context.Context
	client client.Client
	scheme *runtime.Scheme
	config *config.Config
}

// Reconcile reads that state of the cluster for a QuarksStatefulSet object
// and makes changes based on the state read and what is in the QuarksStatefulSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileQuarksStatefulSetStatus) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the QuarksStatefulSet we need to reconcile
	qStatefulSet := &qstsv1a1.QuarksStatefulSet{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling QuarksStatefulSet ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, qStatefulSet)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Debug(ctx, "Skip QuarksStatefulSet reconcile: QuarksStatefulSet not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// get latest statefulSet
	statefulSet, _, err := GetMaxStatefulSetVersion(ctx, r.client, qStatefulSet)
	if err != nil {
		// Reconcile failed due to error - requeue
		return reconcile.Result{}, errors.Wrapf(err, "couldn't get latest StatefulSet")
	}

	if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
		qStatefulSet.Status.Ready = true
		err = r.client.Status().Update(ctx, qStatefulSet)
		if err != nil {
			ctxlog.WithEvent(qStatefulSet, "UpdateStatusError").Errorf(ctx, "Failed to update status on QuarksStatefulSet '%s' (%v): %s", request.NamespacedName, qStatefulSet.ResourceVersion, err)
			return reconcile.Result{Requeue: false}, nil
		}
	}

	return reconcile.Result{}, nil
}
