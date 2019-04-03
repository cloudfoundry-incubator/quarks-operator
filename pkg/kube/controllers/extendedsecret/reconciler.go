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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	esapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
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
	instance := &esapi.ExtendedSecret{}

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

	// Check if secret was already generated
	generatedSecret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.SecretName,
	}
	err = r.client.Get(ctx, namespacedName, generatedSecret)
	if err == nil {
		ctxlog.Info(ctx, "Skip reconcile: secret already exists")
		return reconcile.Result{}, nil
	}

	if apierrors.IsNotFound(err) {
		// Secret doesn't exist yet. Continue reconciling
	} else {
		// Error reading the object - requeue the request.
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, err
	}

	// Create secret
	switch instance.Spec.Type {
	case esapi.Password:
		ctxlog.Info(ctx, "Generating password")
		err = r.createPasswordSecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating password secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating password secret")
		}
	case esapi.RSAKey:
		ctxlog.Info(ctx, "Generating RSA Key")
		err = r.createRSASecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating RSA key secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating RSA key secret")
		}
	case esapi.SSHKey:
		ctxlog.Info(ctx, "Generating SSH Key")
		err = r.createSSHSecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating SSH key secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating SSH key secret")
		}
	case esapi.Certificate:
		ctxlog.Info(ctx, "Generating certificate")
		err = r.createCertificateSecret(ctx, instance)
		if err != nil {
			ctxlog.Info(ctx, "Error generating certificate secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating certificate secret")
		}
	default:
		ctxlog.Infof(ctx, "Invalid type: %s", instance.Spec.Type)
		return reconcile.Result{}, fmt.Errorf("invalid type: %s", instance.Spec.Type)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileExtendedSecret) createPasswordSecret(ctx context.Context, instance *esapi.ExtendedSecret) error {
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

	if err := r.setReference(instance, secret, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for secret '%s' to ExtendedSecret '%s' in namespace '%s'", secret.Name, instance.Name, instance.Namespace)
	}

	return r.client.Create(ctx, secret)
}

func (r *ReconcileExtendedSecret) createRSASecret(ctx context.Context, instance *esapi.ExtendedSecret) error {
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

	if err := r.setReference(instance, secret, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for secret '%s' to ExtendedSecret '%s' in namespace '%s'", secret.Name, instance.Name, instance.Namespace)
	}

	return r.client.Create(ctx, secret)
}

func (r *ReconcileExtendedSecret) createSSHSecret(ctx context.Context, instance *esapi.ExtendedSecret) error {
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

	if err := r.setReference(instance, secret, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for secret '%s' to ExtendedSecret '%s' in namespace '%s'", secret.Name, instance.Name, instance.Namespace)
	}

	return r.client.Create(ctx, secret)
}

func (r *ReconcileExtendedSecret) createCertificateSecret(ctx context.Context, instance *esapi.ExtendedSecret) error {
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

	if err := r.setReference(instance, secret, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for secret '%s' to ExtendedSecret '%s' in namespace '%s'", secret.Name, instance.Name, instance.Namespace)
	}

	return r.client.Create(ctx, secret)
}
