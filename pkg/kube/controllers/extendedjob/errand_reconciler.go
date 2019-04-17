package extendedjob

import (
	"context"
	"reflect"

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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var _ reconcile.Reconciler = &ErrandReconciler{}

// NewErrandReconciler returns a new reconciler for errand jobs
func NewErrandReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	f setOwnerReferenceFunc,
	owner Owner,
) reconcile.Reconciler {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ErrandReconciler{
		ctx:                  ctx,
		client:               mgr.GetClient(),
		config:               config,
		scheme:               mgr.GetScheme(),
		setOwnerReference:    f,
		owner:                owner,
		versionedSecretStore: versionedSecretStore,
	}
}

// ErrandReconciler implements the Reconciler interface
type ErrandReconciler struct {
	ctx                  context.Context
	client               client.Client
	config               *config.Config
	scheme               *runtime.Scheme
	setOwnerReference    setOwnerReferenceFunc
	owner                Owner
	versionedSecretStore versionedsecretstore.VersionedSecretStore
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
			ctxlog.WithEvent(eJob, "UpdateError").Errorf(ctx, "Failed to revert to 'trigger.strategy=manual' on job '%s': %s", eJob.Name, err)
			return result, err
		}
	}

	eJobCopy := eJob.DeepCopy()
	err = r.versionedSecretStore.UpdateSecretReferences(ctx, eJob.GetNamespace(), &eJob.Spec.Template.Spec)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to update update secret references on job '%s': %s", eJob.Name, err)
		return result, err
	}

	if !reflect.DeepEqual(eJob, eJobCopy) {
		err = r.client.Update(ctx, eJob)
		if err != nil {
			ctxlog.Errorf(ctx, "Could not update eJob '%s': ", eJob.GetName(), err)
			return result, errors.Wrapf(err, "could not update eJob '%s'", eJob.GetName())
		}
	}

	// We might want to retrigger an old job due to a config change. In any
	// case, if it's an auto-errand, let's keep track of the referenced
	// configs and make sure the finalizer is in place to clean up ownership.
	if eJob.Spec.UpdateOnConfigChange == true && eJob.Spec.Trigger.Strategy == ejv1.TriggerOnce {
		ctxlog.Debugf(ctx, "Synchronizing ownership on configs for eJob '%s' in namespace", eJob.Name)
		err := r.owner.Sync(ctx, eJob, eJob.Spec.Template.Spec)
		if err != nil {
			return result, errors.Wrapf(err, "could not synchronize ownership for '%s'", eJob.Name)
		}
		if !finalizer.HasFinalizer(eJob) {
			ctxlog.Debugf(ctx, "Add finalizer to extendedJob '%s' in namespace '%s'.", eJob.Name, eJob.Namespace)
			finalizer.AddFinalizer(eJob)
			err = r.client.Update(ctx, eJob)
			if err != nil {
				ctxlog.WithEvent(eJob, "UpdateError").Errorf(ctx, "Could not remove finalizer from eJob '%s': %s", eJob.GetName(), err)
				return reconcile.Result{}, err
			}

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
			ctxlog.WithEvent(eJob, "UpdateError").Errorf(ctx, "Failed to traverse to 'trigger.strategy=done' on job '%s': %s", eJob.Name, err)
			return result, err
		}
	}

	return result, err
}

func (r *ErrandReconciler) createJob(ctx context.Context, eJob ejv1.ExtendedJob) error {
	template := eJob.Spec.Template.DeepCopy()

	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels["ejob-name"] = eJob.Name

	name, err := names.JobName(eJob.Name, "")
	if err != nil {
		return errors.Wrapf(err, "could not generate job name for eJob '%s'", eJob.Name)
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: eJob.Namespace,
			Labels:    map[string]string{"extendedjob": "true"},
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
