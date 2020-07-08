package boshdeployment

import (
	"context"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/reference"
	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// BDPLStateDeployed is the Bosh Deployment Status spec Deployed State
	BDPLStateDeployed = "Deployed"
	// BDPLStateConverting is the Bosh Deployment Status spec State during conversion
	BDPLStateConverting = "Converting to Kube resource"
	// BDPLStateResolving is the Bosh Deployment Status spec during the resolving phase
	BDPLStateResolving = "Resolving Manifest"
)

// NewStatusQSTSReconciler returns a new reconcile.Reconciler for QuarksStatefulSets Status
func NewStatusQSTSReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBoshDeploymentQSTSStatus{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// NewQJobStatusReconciler returns a new reconcile.Reconciler for QuarksStatefulSets Status
func NewQJobStatusReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBoshDeploymentQJobStatus{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// ReconcileBoshDeploymentQSTSStatus reconciles an QuarksStatefulSet object for its status
type ReconcileBoshDeploymentQSTSStatus struct {
	ctx    context.Context
	client client.Client
	scheme *runtime.Scheme
	config *config.Config
}

// ReconcileBoshDeploymentQJobStatus reconciles an QuarksStatefulSet object for its status
type ReconcileBoshDeploymentQJobStatus struct {
	ctx    context.Context
	client client.Client
	scheme *runtime.Scheme
	config *config.Config
}

func resolveDeploymentState(ctx context.Context, client client.Client, bdpl *bdv1.BOSHDeployment) (bool, error) {
	toUpdate := false

	// Get all QJobs from the bdpl
	jobs, err := reference.GetQJobsReferencedBy(ctx, client, *bdpl)
	if err != nil {
		return toUpdate, ctxlog.WithEvent(bdpl, "UpdateStatusError").Errorf(ctx, "Failed to get Qjobs of BDPL (%v): %s", bdpl.Name, err)
	}

	jobsReady := 0

	for _, s := range jobs {
		if s {
			jobsReady++
		}
	}

	// update job counts if necessary
	if bdpl.Status.TotalJobCount != len(jobs) {
		bdpl.Status.TotalJobCount = len(jobs)
		toUpdate = true
	}

	if bdpl.Status.CompletedJobCount != jobsReady {
		bdpl.Status.CompletedJobCount = jobsReady
		toUpdate = true
	}

	// Get all QSTS from the bdpl
	sts, err := reference.GetQSTSReferencedBy(ctx, client, *bdpl)
	if err != nil {
		return toUpdate, ctxlog.WithEvent(bdpl, "UpdateStatusError").Errorf(ctx, "Failed to get Qsts of BDPL (%v): %s", bdpl.Name, err)
	}

	qstsReady := 0

	for _, s := range sts {
		if s {
			qstsReady++
		}
	}

	// update Instance groups count if necessary
	if bdpl.Status.TotalInstanceGroups != len(sts) {
		bdpl.Status.TotalInstanceGroups = len(sts)
		toUpdate = true
	}

	if bdpl.Status.DeployedInstanceGroups != qstsReady {
		bdpl.Status.DeployedInstanceGroups = qstsReady
		toUpdate = true
	}

	// Computing BDPL final State
	// Converting state: Job are finished, but instance groups are not.
	// 					 or either way around
	convertingState := (bdpl.Status.CompletedJobCount == bdpl.Status.TotalJobCount &&
		bdpl.Status.TotalInstanceGroups != bdpl.Status.DeployedInstanceGroups) || (bdpl.Status.CompletedJobCount != bdpl.Status.TotalJobCount &&
		bdpl.Status.TotalInstanceGroups == bdpl.Status.DeployedInstanceGroups)

	// Resolving state: Neither jobs or instance group are ready
	resolvingState := bdpl.Status.CompletedJobCount != bdpl.Status.TotalJobCount &&
		bdpl.Status.TotalInstanceGroups != bdpl.Status.DeployedInstanceGroups

	// Deployed state: Jobs and instance groups are completed
	deployedState := bdpl.Status.CompletedJobCount == bdpl.Status.TotalJobCount &&
		bdpl.Status.TotalInstanceGroups == bdpl.Status.DeployedInstanceGroups

	if convertingState && bdpl.Status.State != BDPLStateConverting {
		bdpl.Status.State = BDPLStateConverting
		toUpdate = true
	}

	if resolvingState && bdpl.Status.State != BDPLStateResolving {
		bdpl.Status.State = BDPLStateResolving
		toUpdate = true
	}

	if deployedState && bdpl.Status.State != BDPLStateDeployed {
		bdpl.Status.State = BDPLStateDeployed
		toUpdate = true
	}
	return toUpdate, nil
}

// Reconcile reads that state of QuarksJobs and QuarksStatefulSets and updates the bosh deployment status accordingly.
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBoshDeploymentQJobStatus) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the QuarksStatefulSet we need to reconcile
	qJob := &qjv1a1.QuarksJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling Bosh Deployment from qjob ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, qJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Debug(ctx, "Skip qJob reconcile: qJob not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	deploymentName, ok := qJob.GetLabels()[bdv1.LabelDeploymentName]
	if !ok {
		return reconcile.Result{Requeue: false},
			ctxlog.WithEvent(qJob, "LabelMissingError").Errorf(ctx, "There's no label for a BoshDeployment name on the QSTS '%s'", request.NamespacedName)
	}
	bdpl := &bdv1.BOSHDeployment{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: deploymentName}, bdpl)
	if err != nil {
		return reconcile.Result{Requeue: false},
			ctxlog.WithEvent(qJob, "GetBOSHDeployment").Errorf(ctx, "Failed to get BoshDeployment instance '%s/%s': %v", request.Namespace, deploymentName, err)
	}

	toUpdate, err := resolveDeploymentState(r.ctx, r.client, bdpl)
	if err != nil {
		return reconcile.Result{Requeue: false}, err
	}

	if toUpdate {
		now := metav1.Now()
		bdpl.Status.StateTimestamp = &now

		err = r.client.Status().Update(ctx, bdpl)
		if err != nil {
			return reconcile.Result{Requeue: false}, ctxlog.WithEvent(bdpl, "UpdateStatusError").Errorf(ctx, "Failed to update status on BDPL '%s' (%v): %s", request.NamespacedName, bdpl.ResourceVersion, err)
		}
	}

	return reconcile.Result{}, nil
}

// Reconcile reads that state of QuarksJobs and QuarksStatefulSets and updates the bosh deployment status accordingly.
// and makes changes based on the state read and what is in the QuarksStatefulSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBoshDeploymentQSTSStatus) Reconcile(request reconcile.Request) (reconcile.Result, error) {

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

	deploymentName, ok := qStatefulSet.GetLabels()[bdv1.LabelDeploymentName]
	if !ok {
		return reconcile.Result{Requeue: false},
			ctxlog.WithEvent(qStatefulSet, "LabelMissingError").Errorf(ctx, "There's no label for a BoshDeployment name on the QSTS '%s'", request.NamespacedName)
	}

	bdpl := &bdv1.BOSHDeployment{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: deploymentName}, bdpl)
	if err != nil {
		return reconcile.Result{Requeue: false},
			ctxlog.WithEvent(qStatefulSet, "GetBOSHDeployment").Errorf(ctx, "Failed to get BoshDeployment instance '%s/%s': %v", request.Namespace, deploymentName, err)
	}

	toUpdate, err := resolveDeploymentState(r.ctx, r.client, bdpl)
	if err != nil {
		return reconcile.Result{Requeue: false}, err
	}

	if toUpdate {
		now := metav1.Now()
		bdpl.Status.StateTimestamp = &now
		err = r.client.Status().Update(ctx, bdpl)
		if err != nil {
			return reconcile.Result{Requeue: false}, ctxlog.WithEvent(bdpl, "UpdateStatusError").Errorf(ctx, "Failed to update status on BDPL '%s' (%v): %s", request.NamespacedName, bdpl.ResourceVersion, err)
		}
	}

	return reconcile.Result{}, nil
}

// AddBDPLStatusReconciler creates a new BDPL Status controller to update BDPL status.
func AddBDPLStatusReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "quarks-bdpl-status-reconciler", mgr.GetEventRecorderFor("quarks-bdpl-status-recorder"))
	r := NewStatusQSTSReconciler(ctx, config, mgr)
	rjobs := NewQJobStatusReconciler(ctx, config, mgr)

	// Create a new controller
	c, err := controller.New("quarks-bdpl-qsts-status-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding StatusQSTSReconciler controller to manager failed.")
	}

	// Create a new controller
	cjobs, err := controller.New("quarks-bdpl-qjobs-status-controller", mgr, controller.Options{
		Reconciler:              rjobs,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding StatusQJobsReconciler controller to manager failed.")
	}

	// doamins are watched on updates too to get status changes
	certPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &qstsv1a1.QuarksStatefulSet{}}, &handler.EnqueueRequestForObject{}, certPred)
	if err != nil {
		return errors.Wrapf(err, "Watching QSTS in QuarksBDPLStatus controller failed.")
	}

	err = cjobs.Watch(&source.Kind{Type: &qjv1a1.QuarksJob{}}, &handler.EnqueueRequestForObject{}, certPred)
	if err != nil {
		return errors.Wrapf(err, "Watching QJobs in QuarksBDPLStatus controller failed.")
	}

	return nil
}
