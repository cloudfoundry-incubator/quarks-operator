Spackage extendedjob

import (
	"context"
	"fmt"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &ErrandReconciler{}

// NewErrandReconciler returns a new reconciler for errand jobs
func NewErrandReconciler(
	log *zap.SugaredLogger,
	ctrConfig *controllersconfig.ControllersConfig,
	mgr manager.Manager,
	f setOwnerReferenceFunc,
) reconcile.Reconciler {

	errandReconcilerLog := log.Named("extendedjob-errand-reconciler")
        errandReconcilerLog.Info("Creating a reconciler for errand ExtendedJobs")

	return &ErrandReconciler{
		client:            mgr.GetClient(),
		log:               errandReconcilerLog,
		ctrConfig:         ctrConfig,
		recorder:          mgr.GetRecorder("extendedjob errand reconciler"),
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
	}
}

// ErrandReconciler implements the Reconciler interface
type ErrandReconciler struct {
	client            client.Client
	log               *zap.SugaredLogger
	ctrConfig         *controllersconfig.ControllersConfig
	recorder          record.EventRecorder
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
}

// Reconcile starts jobs for extended jobs of the type errand with Run being set to 'now' manually
func (r *ErrandReconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	extJob := &ejv1.ExtendedJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := controllersconfig.NewBackgroundContextWithTimeout(r.ctrConfig.CtxType, r.ctrConfig.CtxTimeOut)
	defer cancel()

	err = r.client.Get(ctx, request.NamespacedName, extJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, extended job is probably deleted
			r.log.Infof("Failed to find extended job '%s', not retrying: %s", request.NamespacedName, err)
			err = nil
			return
		}
		// Error reading the object - requeue the request.
		r.log.Errorf("Failed to get the extended job '%s': %s", request.NamespacedName, err)
		return
	}

	if extJob.Spec.Run == ejv1.RunNow {
		// set Run back to manually for errand jobs
		extJob.Spec.Run = ejv1.RunManually
		err = r.client.Update(ctx, extJob)
		if err != nil {
			r.log.Errorf("Failed to revert to 'Run=manually' on job '%s': %s", extJob.Name, err)
			return
		}
	}

	err = r.createJob(ctx, *extJob)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			r.log.Infof("Skip '%s' triggered manually: already running", extJob.Name)
			// we don't want to requeue the job
			err = nil
		} else {
			r.log.Errorf("Failed to create job '%s': %s", extJob.Name, err)
		}
		return
	}
	r.log.Infof("Created errand job for '%s'", extJob.Name)

	return
}

func (r *ErrandReconciler) createJob(ctx context.Context, extJob ejv1.ExtendedJob) error {
	name := fmt.Sprintf("job-%s-%s", truncate(extJob.Name, 30), randSuffix(extJob.Name))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: extJob.Namespace,
			Labels:    map[string]string{"extendedjob": "true"},
		},
		Spec: batchv1.JobSpec{Template: extJob.Spec.Template},
	}

	err := r.setOwnerReference(&extJob, job, r.scheme)
	if err != nil {
		r.log.Errorf("Failed to set owner reference on job for '%s': %s", extJob.Name, err)
	}

	err = r.client.Create(ctx, job)
	if err != nil {
		return err
	}

	return nil
}
