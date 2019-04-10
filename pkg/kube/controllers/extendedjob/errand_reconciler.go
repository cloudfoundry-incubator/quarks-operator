package extendedjob

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	store "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
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
	versionedSecretStore := store.NewVersionedSecretStore(mgr.GetClient())

	return &ErrandReconciler{
		ctx:                  ctx,
		client:               mgr.GetClient(),
		config:               config,
		recorder:             mgr.GetRecorder("extendedjob errand reconciler"),
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
	recorder             record.EventRecorder
	scheme               *runtime.Scheme
	setOwnerReference    setOwnerReferenceFunc
	owner                Owner
	versionedSecretStore store.VersionedSecretStore
}

// Reconcile starts jobs for extended jobs of the type errand with Run being set to 'now' manually
func (r *ErrandReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var result = reconcile.Result{}
	extJob := &ejv1.ExtendedJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling errand job ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, extJob)
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

	if extJob.Spec.Trigger.Strategy == ejv1.TriggerNow {
		// set Strategy back to manual for errand jobs
		extJob.Spec.Trigger.Strategy = ejv1.TriggerManual
		err = r.client.Update(ctx, extJob)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to revert to 'trigger.strategy=manual' on job '%s': %s", extJob.Name, err)
			return result, err
		}
	}

	err = r.updateSecretReference(ctx, extJob)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to update update secret references on job '%s': %s", extJob.Name, err)
		return result, err
	}

	// We might want to retrigger an old job due to a config change. In any
	// case, if it's an auto-errand, let's keep track of the referenced
	// configs and make sure the finalizer is in place to clean up ownership.
	if extJob.Spec.UpdateOnConfigChange == true && extJob.Spec.Trigger.Strategy == ejv1.TriggerOnce {
		ctxlog.Debugf(ctx, "Synchronizing ownership on configs for extJob '%s' in namespace", extJob.Name)
		err := r.owner.Sync(ctx, extJob, extJob.Spec.Template.Spec)
		if err != nil {
			return result, errors.Wrapf(err, "could not synchronize ownership for '%s'", extJob.Name)
		}
		if !finalizer.HasFinalizer(extJob) {
			ctxlog.Debugf(ctx, "Add finalizer to extendedJob '%s' in namespace '%s'.", extJob.Name, extJob.Namespace)
			finalizer.AddFinalizer(extJob)
			err = r.client.Update(ctx, extJob)
			if err != nil {
				ctxlog.Errorf(ctx, "Could not remove finalizer from ExtJob '%s': ", extJob.GetName(), err)
				return reconcile.Result{}, err
			}

		}
	}

	err = r.createJob(ctx, *extJob)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctxlog.Infof(ctx, "Skip '%s' triggered manually: already running", extJob.Name)
			// we don't want to requeue the job
			err = nil
		} else {
			ctxlog.Errorf(ctx, "Failed to create job '%s': %s", extJob.Name, err)
		}
		return result, err
	}
	ctxlog.Infof(ctx, "Created errand job for '%s'", extJob.Name)

	if extJob.Spec.Trigger.Strategy == ejv1.TriggerOnce {
		// traverse Strategy into the final 'done' state
		extJob.Spec.Trigger.Strategy = ejv1.TriggerDone
		err = r.client.Update(ctx, extJob)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to traverse to 'trigger.strategy=done' on job '%s': %s", extJob.Name, err)
			return result, err
		}
	}

	return result, err
}

func (r *ErrandReconciler) createJob(ctx context.Context, extJob ejv1.ExtendedJob) error {
	template := extJob.Spec.Template.DeepCopy()

	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels["ejob-name"] = extJob.Name

	name, err := names.JobName(extJob.Name, "")
	if err != nil {
		return errors.Wrapf(err, "could not generate job name for extJob '%s'", extJob.Name)
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: extJob.Namespace,
			Labels:    map[string]string{"extendedjob": "true"},
		},
		Spec: batchv1.JobSpec{Template: *template},
	}

	err = r.setOwnerReference(&extJob, job, r.scheme)
	if err != nil {
		ctxlog.Errorf(ctx, "failed to set owner reference on job for '%s': %s", extJob.Name, err)
		return err
	}

	err = r.client.Create(ctx, job)
	if err != nil {
		return err
	}

	return nil
}

// updateSecretReference look and see if any referenced secret is versioned, and if it is, but it's not latest, you have to change the reference
func (r *ErrandReconciler) updateSecretReference(ctx context.Context, extJob *ejv1.ExtendedJob) error {
	var err error
	extJobCopy := extJob.DeepCopy()

	_, secretsInSpec := owner.GetConfigNamesFromSpec(extJob.Spec.Template.Spec)
	for secretNameInSpec := range secretsInSpec {
		secretKey := types.NamespacedName{Namespace: extJob.GetNamespace(), Name: secretNameInSpec}
		secret := &corev1.Secret{}
		err = r.client.Get(ctx, secretKey, secret)
		if err != nil && apierrors.IsNotFound(err) {
			// Replace a new versioned secret
			ctxlog.Debugf(ctx, "Getting latest secret '%s'", secretNameInSpec)
			versionedSecret, err := r.versionedSecretStore.Latest(ctx, extJob.GetNamespace(), secretNameInSpec)
			if err != nil && apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "versioned secret %s/%s doesn't exist", extJob.GetNamespace(), secretNameInSpec)
				continue
			} else if err != nil {
				return errors.Wrapf(err, "failed to get versioned secret %s/%s", extJob.GetNamespace(), secretNameInSpec)
			}

			owner.ReplaceVolumesSecretRef(extJob.Spec.Template.Spec.Volumes, secretNameInSpec, versionedSecret.GetName())
			owner.ReplaceContainerEnvsSecretRef(extJob.Spec.Template.Spec.Containers, secretNameInSpec, versionedSecret.GetName())
		} else if err != nil {
			return errors.Wrapf(err, "failed to retrieve secret %s/%s", extJob.GetNamespace(), secretNameInSpec)
		}

		// Update versioned secret if there is a newer version
		secretLabels := secret.GetLabels()
		if secretLabels == nil {
			continue
		}
		referencedSecretName := secretLabels[ejv1.LabelReferencedSecretName]

		ctxlog.Debugf(ctx, "Getting latest secret '%s'", referencedSecretName)
		versionedSecret, err := r.versionedSecretStore.Latest(ctx, extJob.GetNamespace(), referencedSecretName)
		if err != nil && apierrors.IsNotFound(err) {
			return fmt.Errorf("versioned secret %s/%s doesn't exist", extJob.GetNamespace(), referencedSecretName)
		} else if err != nil {
			return errors.Wrapf(err, "failed to get versioned secret %s/%s", extJob.GetNamespace(), referencedSecretName)
		}

		if referencedSecretName != versionedSecret.GetName() {
			owner.ReplaceVolumesSecretRef(extJob.Spec.Template.Spec.Volumes, secretNameInSpec, versionedSecret.GetName())
			owner.ReplaceContainerEnvsSecretRef(extJob.Spec.Template.Spec.Containers, secretNameInSpec, versionedSecret.GetName())
		}
	}

	if extJobCopy != extJob {
		err = r.client.Update(ctx, extJob)
		if err != nil {
			ctxlog.Errorf(ctx, "Could not update ExtJob '%s': ", extJob.GetName(), err)
			return err
		}
	}

	return nil
}
