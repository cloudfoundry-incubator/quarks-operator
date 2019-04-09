package extendedsecret

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	corev1 "k8s.io/api/core/v1"
)

// NewVersionedSecretReconciler returns a new Reconciler for versioned secret
func NewVersionedSecretReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	versionedSecretStore := NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileVersionedSecret{
		ctx:                  ctx,
		client:               mgr.GetClient(),
		config:               config,
		scheme:               mgr.GetScheme(),
		versionedSecretStore: versionedSecretStore,
	}
}

// ReconcileVersionedSecret reconciles an Secret object
type ReconcileVersionedSecret struct {
	ctx                  context.Context
	client               client.Client
	config               *config.Config
	scheme               *runtime.Scheme
	versionedSecretStore VersionedSecretStore
}

// Reconcile reads that state of the secret
func (r *ReconcileVersionedSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling versioned secret %s", request.NamespacedName)

	// Fetch the Secret
	secret := &corev1.Secret{}

	err := r.client.Get(ctx, request.NamespacedName, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Debug(ctx, "Skip reconcile: secret not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get secret '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, errors.Wrapf(err, "could not get secret '%s'", request.NamespacedName)
	}

	// Find dependantJob and dependantSecretName
	dependantJob, dependantSecretName, err := r.findDependant(ctx, secret)
	if err != nil {
		return reconcile.Result{Requeue: true}, errors.Wrapf(err, "could not find dependant of secret '%s'", request.NamespacedName)
	}

	err = r.applyVersionedSecret(ctx, secret, dependantJob, dependantSecretName)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to apply versioned secret '%s/%s' to its dependant: %v", secret.GetNamespace(), secret.GetName(), err)
		return reconcile.Result{}, errors.Wrapf(err, "could not apply versioned secret '%s/%s' to its dependant", secret.GetNamespace(), secret.GetName())
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVersionedSecret) findDependant(ctx context.Context, secret *corev1.Secret) (*ejv1.ExtendedJob, string, error) {
	var dependantSecretName string
	exJob := &ejv1.ExtendedJob{}

	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		ctxlog.Errorf(ctx, "secret '%s' does not have labels", secret.GetName())
		return exJob, dependantSecretName, fmt.Errorf("secret '%s' does not have labels", secret.GetName())
	}

	dependantName, ok := secretLabels[ejv1.LabelDependantJobName]
	if !ok {
		ctxlog.Debugf(ctx, "label '%s' not found in versioned secret '%s'", ejv1.LabelDependantJobName, secret.GetName())
		return exJob, dependantSecretName, fmt.Errorf("label '%s' not found in versioned secret '%s'", ejv1.LabelDependantJobName, secret.GetName())
	}

	dependantSecretName, ok = secretLabels[ejv1.LabelDependantSecretName]
	if !ok {
		ctxlog.Errorf(ctx, "label '%s' not found in versioned secret '%s'", ejv1.LabelDependantSecretName, secret.GetName())
		return exJob, dependantSecretName, fmt.Errorf("label '%s' not found in versioned secret '%s'", ejv1.LabelDependantSecretName, secret.GetName())
	}

	exJobKey := types.NamespacedName{Name: dependantName, Namespace: secret.GetNamespace()}

	err := r.client.Get(ctx, exJobKey, exJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Debug(ctx, "Reconcile: dependant job not found")
			return exJob, dependantSecretName, errors.Wrapf(err, "dependant job '%s' not found", exJobKey.Name)
		}
		ctxlog.Errorf(ctx, "Failed to get dependant job '%s': %v", dependantName, err)
		return exJob, dependantSecretName, errors.Wrapf(err, "could not get dependant job '%s'", exJobKey.Name)
	}

	return exJob, dependantSecretName, nil
}

func (r *ReconcileVersionedSecret) applyVersionedSecret(ctx context.Context, secret *corev1.Secret, dependantJob *ejv1.ExtendedJob, dependantSecretName string) error {
	secretLabels := secret.GetLabels()
	versionStr, ok := secretLabels[LabelVersion]
	if !ok {
		return fmt.Errorf("label '%s' found in versioned secret '%s'", ejv1.LabelDependantSecretName, secret.GetName())
	}
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return errors.Wrapf(err, "could not convert version '%s' to int", versionStr)
	}

	ctxlog.Debugf(ctx, "Updating secret to ExtendedJob '%s/%s' volumes", dependantJob.GetNamespace(), dependantJob.GetName())
	err = r.updateExtendedJobVolumes(ctx, secret.GetNamespace(), version, dependantJob, dependantSecretName)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to update ExtendedJob '%s/%s' Volumes: %v", dependantJob.GetNamespace(), dependantJob.GetName(), err)
		return err
	}

	return nil
}

func (r *ReconcileVersionedSecret) updateExtendedJobVolumes(ctx context.Context, namespace string, version int, extendedJob *ejv1.ExtendedJob, secretName string) error {
	extendedJobCopy := extendedJob.DeepCopy()

	volumes := extendedJob.Spec.Template.Spec.Volumes
	for idx, vol := range volumes {
		if vol.VolumeSource.Secret != nil && vol.VolumeSource.Secret.SecretName == secretName {
			secret, err := r.versionedSecretStore.Get(ctx, namespace, secretName, version)
			if err != nil {
				ctxlog.Errorf(ctx, "Could not to get version '%d' of secret '%s': %v", version, secretName, err)
				return errors.Wrapf(err, "could not to get secret '%s'", secretName)
			}

			ctxlog.Debugf(ctx, "Changing volume secret from '%s' to '%s'", vol.VolumeSource.Secret.SecretName, secret.GetName())
			vol.VolumeSource.Secret.SecretName = secret.GetName()
			volumes[idx] = vol
		}
	}

	if !reflect.DeepEqual(volumes, extendedJobCopy.Spec.Template.Spec.Volumes) {
		ctxlog.Debugf(ctx, "Updating ExtendedJob '%s/%s'", extendedJob.GetNamespace(), extendedJob.GetName())
		err := r.client.Update(ctx, extendedJob)
		if err != nil {
			ctxlog.Errorf(ctx, "Could not update ExtendedJob '%s/%s': %v", extendedJob.GetNamespace(), extendedJob.GetName(), err)
			return errors.Wrapf(err, "could not update ExtendedJob '%s/%s'", extendedJob.GetNamespace(), extendedJob.GetName())
		}
	}

	return nil
}
