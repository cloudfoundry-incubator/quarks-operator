package extendedjob

import (
	"context"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
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
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var _ reconcile.Reconciler = &ErrandReconciler{}

// NewErrandReconciler returns a new reconciler for errand jobs
func NewErrandReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	f setOwnerReferenceFunc,
	store vss.VersionedSecretStore,
) reconcile.Reconciler {
	return &ErrandReconciler{
		ctx:                  ctx,
		client:               mgr.GetClient(),
		config:               config,
		scheme:               mgr.GetScheme(),
		setOwnerReference:    f,
		versionedSecretStore: store,
	}
}

// ErrandReconciler implements the Reconciler interface
type ErrandReconciler struct {
	ctx                  context.Context
	client               client.Client
	config               *config.Config
	scheme               *runtime.Scheme
	setOwnerReference    setOwnerReferenceFunc
	versionedSecretStore vss.VersionedSecretStore
}

// Reconcile starts jobs for extended jobs of the type errand with Run being set to 'now' manually
func (r *ErrandReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var result = reconcile.Result{}
	eJob := &ejv1.ExtendedJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling errand job ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, eJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, extended job is probably deleted
			ctxlog.Infof(ctx, "Failed to find extended job '%s', not retrying: %s", request.NamespacedName, err)
			err = nil
			return result, err
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get the extended job '%s': %s", request.NamespacedName, err)
		return result, err
	}

	if eJob.Spec.Trigger.Strategy == ejv1.TriggerNow {
		// set Strategy back to manual for errand jobs
		eJob.Spec.Trigger.Strategy = ejv1.TriggerManual
		err = r.client.Update(ctx, eJob)
		if err != nil {
			err = ctxlog.WithEvent(eJob, "UpdateError").Errorf(ctx, "Failed to revert to 'trigger.strategy=manual' on job '%s': %s", eJob.Name, err)
			return result, err
		}
	}

	err = r.createJob(ctx, *eJob)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctxlog.WithEvent(eJob, "AlreadyRunning").Infof(ctx, "Skip '%s' triggered manually: already running", eJob.Name)
			// we don't want to requeue the job
			err = nil
		} else {
			ctxlog.WithEvent(eJob, "CreateJobError").Errorf(ctx, "Failed to create job '%s': %s", eJob.Name, err)
		}
		return result, err
	}
	ctxlog.WithEvent(eJob, "CreateJob").Infof(ctx, "Created errand job for '%s'", eJob.Name)

	if eJob.Spec.Trigger.Strategy == ejv1.TriggerOnce {
		// traverse Strategy into the final 'done' state
		eJob.Spec.Trigger.Strategy = ejv1.TriggerDone
		err = r.client.Update(ctx, eJob)
		if err != nil {
			err = ctxlog.WithEvent(eJob, "UpdateError").Errorf(ctx, "Failed to traverse to 'trigger.strategy=done' on job '%s': %s", eJob.Name, err)
			return reconcile.Result{Requeue: false}, err
		}
	}

	return result, err
}

func (r *ErrandReconciler) createJob(ctx context.Context, eJob ejv1.ExtendedJob) error {
	template := eJob.Spec.Template.DeepCopy()

	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels[ejv1.LabelEJobName] = eJob.Name

	r.versionedSecretStore.SetSecretReferences(ctx, eJob.Namespace, &template.Spec)

	name, err := names.JobName(eJob.Name, "")
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
		ctxlog.WithEvent(&eJob, "SetOwnerReferenceError").Errorf(ctx, "failed to set owner reference on job for '%s': %s", eJob.Name, err)
		return err
	}

	err = r.client.Create(ctx, job)
	if err != nil {
		return err
	}

	return nil
}
