package extendedsecret

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewReconciler returns a new Reconciler
func NewReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, generator credsgen.Generator, srf setReferenceFunc) reconcile.Reconciler {
	return &ReconcileExtendedSecret{
		ctx:          ctx,
		config:       config,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		generator:    generator,
		setReference: srf,
	}
}

// ReconcileExtendedSecret reconciles an ExtendedSecret object
type ReconcileExtendedSecret struct {
	ctx          context.Context
	client       client.Client
	generator    credsgen.Generator
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	config       *config.Config
}

// Reconcile reads that state of the cluster for a ExtendedSecret object and makes changes based on the state read
// and what is in the ExtendedSecret.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileExtendedSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &esv1.ExtendedSecret{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling ExtendedSecret %s", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Info(ctx, "Skip reconcile: CRD not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, err
	}

	// Check if secret could be generated when secret was already created
	canBeGenerated, err := r.canBeGenerated(ctx, instance)
	if err != nil {
		ctxlog.Errorf(ctx, "Error reading the secret: %v", err.Error())
		return reconcile.Result{}, err
	}
	if !canBeGenerated {
		ctxlog.WithEvent(instance, "SkipReconcile").Infof(ctx, "Skip reconcile: secret '%s' already exists and it's not generated", instance.Spec.SecretName)
		return reconcile.Result{}, nil
	}

	// Create secret
	switch instance.Spec.Type {
	case esv1.Password:
		ctxlog.Info(ctx, "Generating password")
		err = r.createPasswordSecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating password secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating password secret")
		}
	case esv1.RSAKey:
		ctxlog.Info(ctx, "Generating RSA Key")
		err = r.createRSASecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating RSA key secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating RSA key secret")
		}
	case esv1.SSHKey:
		ctxlog.Info(ctx, "Generating SSH Key")
		err = r.createSSHSecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating SSH key secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating SSH key secret")
		}
	case esv1.Certificate:
		ctxlog.Info(ctx, "Generating certificate")
		err = r.createCertificateSecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating certificate secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating certificate secret")
		}
	default:
		err = ctxlog.WithEvent(instance, "InvalidTypeError").Errorf(ctx, "Invalid type: %s", instance.Spec.Type)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileExtendedSecret) createPasswordSecret(ctx context.Context, instance *esv1.ExtendedSecret) error {
	request := credsgen.PasswordGenerationRequest{}
	password := r.generator.GeneratePassword(instance.GetName(), request)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.SecretName,
			Namespace: instance.GetNamespace(),
		},
		StringData: map[string]string{
			"password": password,
		},
	}

	return r.createSecret(ctx, instance, secret)
}

func (r *ReconcileExtendedSecret) createRSASecret(ctx context.Context, instance *esv1.ExtendedSecret) error {
	key, err := r.generator.GenerateRSAKey(instance.GetName())
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.SecretName,
			Namespace: instance.GetNamespace(),
		},
		Data: map[string][]byte{
			"private_key": key.PrivateKey,
			"public_key":  key.PublicKey,
		},
	}

	return r.createSecret(ctx, instance, secret)
}

func (r *ReconcileExtendedSecret) createSSHSecret(ctx context.Context, instance *esv1.ExtendedSecret) error {
	key, err := r.generator.GenerateSSHKey(instance.GetName())
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.SecretName,
			Namespace: instance.GetNamespace(),
		},
		Data: map[string][]byte{
			"private_key":            key.PrivateKey,
			"public_key":             key.PublicKey,
			"public_key_fingerprint": []byte(key.Fingerprint),
		},
	}

	return r.createSecret(ctx, instance, secret)
}

func (r *ReconcileExtendedSecret) createCertificateSecret(ctx context.Context, instance *esv1.ExtendedSecret) error {
	var request credsgen.CertificateGenerationRequest
	if instance.Spec.Request.CertificateRequest.IsCA {
		// Generate self-signed root CA certificate
		request = credsgen.CertificateGenerationRequest{
			IsCA:       instance.Spec.Request.CertificateRequest.IsCA,
			CommonName: instance.Spec.Request.CertificateRequest.CommonName,
		}
	} else {
		// Get CA certificate
		caSecret := &corev1.Secret{}
		caNamespacedName := types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      instance.Spec.Request.CertificateRequest.CARef.Name,
		}
		err := r.client.Get(ctx, caNamespacedName, caSecret)
		if err != nil {
			return errors.Wrap(err, "getting CA secret")
		}
		ca := caSecret.Data[instance.Spec.Request.CertificateRequest.CARef.Key]

		// Get CA key
		if instance.Spec.Request.CertificateRequest.CAKeyRef.Name != instance.Spec.Request.CertificateRequest.CARef.Name {
			caSecret = &corev1.Secret{}
			caNamespacedName = types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      instance.Spec.Request.CertificateRequest.CAKeyRef.Name,
			}
			err = r.client.Get(ctx, caNamespacedName, caSecret)
			if err != nil {
				return errors.Wrap(err, "getting CA Key secret")
			}
		}
		key := caSecret.Data[instance.Spec.Request.CertificateRequest.CAKeyRef.Key]

		// Build the generation request
		request = credsgen.CertificateGenerationRequest{
			IsCA:             instance.Spec.Request.CertificateRequest.IsCA,
			CommonName:       instance.Spec.Request.CertificateRequest.CommonName,
			AlternativeNames: instance.Spec.Request.CertificateRequest.AlternativeNames,
			CA: credsgen.Certificate{
				IsCA:        true,
				PrivateKey:  key,
				Certificate: ca,
			},
		}
	}

	// Generate certificate
	cert, err := r.generator.GenerateCertificate(instance.GetName(), request)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.SecretName,
			Namespace: instance.GetNamespace(),
		},
		Data: map[string][]byte{
			"certificate": cert.Certificate,
			"private_key": cert.PrivateKey,
			"is_ca":       []byte(strconv.FormatBool(instance.Spec.Request.CertificateRequest.IsCA)),
		},
	}

	if len(request.CA.Certificate) > 0 {
		secret.Data["ca"] = request.CA.Certificate
	}

	return r.createSecret(ctx, instance, secret)
}

func (r *ReconcileExtendedSecret) canBeGenerated(ctx context.Context, instance *esv1.ExtendedSecret) (bool, error) {
	secretName := instance.Spec.SecretName

	existingSecret := &corev1.Secret{}
	err := r.client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: instance.GetNamespace()}, existingSecret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return true, errors.Wrapf(err, "could not get secret")
	}

	secretLabels := existingSecret.GetLabels()
	if secretLabels == nil {
		secretLabels = map[string]string{}
	}

	if secretLabels[esv1.LabelKind] != esv1.GeneratedSecretKind {
		return false, nil
	}

	return true, nil
}

// createSecret applies common properties(labels and ownerReferences) to the secret and creates it
func (r *ReconcileExtendedSecret) createSecret(ctx context.Context, instance *esv1.ExtendedSecret, secret *corev1.Secret) error {
	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		secretLabels = map[string]string{}
	}

	secretLabels[esv1.LabelKind] = esv1.GeneratedSecretKind

	secret.SetLabels(secretLabels)

	if err := r.setReference(instance, secret, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for secret '%s' to ExtendedSecret '%s' in namespace '%s'", secret.GetName(), instance.GetName(), instance.GetNamespace())
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.client, secret.DeepCopy(), func(obj runtime.Object) error {
		s, ok := obj.(*corev1.Secret)
		if !ok {
			return fmt.Errorf("object is not a Secret")
		}
		secret.DeepCopyInto(s)
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "could not create or update secret '%s'", secret.GetName())
	}

	ctxlog.Debugf(ctx, "Secret '%s' has been %s", secret.Name, op)

	return nil
}
