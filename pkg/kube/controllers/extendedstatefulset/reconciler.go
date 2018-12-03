package extendedstatefulset

import (
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Check that ReconcileExtendedStatefulSet implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileExtendedStatefulSet{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(log *zap.SugaredLogger, mgr manager.Manager, srf setReferenceFunc) reconcile.Reconciler {
	return &ReconcileExtendedStatefulSet{
		log:          log,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		setReference: srf,
	}
}

// ReconcileExtendedStatefulSet reconciles an ExtendedStatefulSet object
type ReconcileExtendedStatefulSet struct {
	client       client.Client
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	log          *zap.SugaredLogger
}

// Reconcile reads that state of the cluster for a ExtendedStatefulSet object
// and makes changes based on the state read and what is in the ExtendedStatefulSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileExtendedStatefulSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Infof("Reconciling ExtendedStatefulSet %s\n", request.NamespacedName)
	return reconcile.Result{}, nil
}
