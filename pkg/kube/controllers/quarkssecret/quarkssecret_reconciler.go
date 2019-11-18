package quarkssecret

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	certv1 "k8s.io/api/certificates/v1beta1"
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
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewQuarksSecretReconciler returns a new ReconcileQuarksSecret
func NewQuarksSecretReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, generator credsgen.Generator, srf setReferenceFunc) reconcile.Reconciler {
	return &ReconcileQuarksSecret{
		ctx:          ctx,
		config:       config,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		generator:    generator,
		setReference: srf,
	}
}

// ReconcileQuarksSecret reconciles an QuarksSecret object
type ReconcileQuarksSecret struct {
	ctx          context.Context
	client       client.Client
	generator    credsgen.Generator
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	config       *config.Config
}

type caNotReadyError struct {
	message string
}

func newCaNotReadyError(message string) *caNotReadyError {
	return &caNotReadyError{message: message}
}

// Error returns the error message
func (e *caNotReadyError) Error() string {
	return e.message
}

func isCaNotReady(o interface{}) bool {
	err := o.(error)
	err = errors.Cause(err)
	_, ok := err.(*caNotReadyError)
	return ok
}

// Reconcile reads that state of the cluster for a QuarksSecret object and makes changes based on the state read
// and what is in the QuarksSecret.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileQuarksSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &qsv1a1.QuarksSecret{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling QuarksSecret %s", request.NamespacedName)
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
		return reconcile.Result{}, errors.Wrap(err, "Error reading quarksSecret")
	}

	if meltdown.NewWindow(r.config.MeltdownDuration, instance.Status.LastReconcile).Contains(time.Now()) {
		ctxlog.WithEvent(instance, "Meltdown").Debugf(ctx, "Resource '%s' is in meltdown, requeue reconcile after %s", instance.Name, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	}

	// Check if secret could be generated when secret was already created
	canBeGenerated, err := r.canBeGenerated(ctx, instance)
	if err != nil {
		ctxlog.Errorf(ctx, "Error reading the secret: %v", err.Error())
		return reconcile.Result{}, err
	}
	if !canBeGenerated {
		ctxlog.WithEvent(instance, "SkipReconcile").Infof(ctx, "Skip reconcile: quarksSecret '%s' is already generated", instance.Name)
		return reconcile.Result{}, nil
	}

	// Create secret
	switch instance.Spec.Type {
	case qsv1a1.Password:
		ctxlog.Info(ctx, "Generating password")
		err = r.createPasswordSecret(ctx, instance)
		if err != nil {
			ctxlog.Infof(ctx, "Error generating password secret: %s", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating password secret failed.")
		}
	case qsv1a1.RSAKey:
		ctxlog.Info(ctx, "Generating RSA Key")
		err = r.createRSASecret(ctx, instance)
		if err != nil {
			ctxlog.Infof(ctx, "Error generating RSA key secret: %s", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating RSA key secret failed.")
		}
	case qsv1a1.SSHKey:
		ctxlog.Info(ctx, "Generating SSH Key")
		err = r.createSSHSecret(ctx, instance)
		if err != nil {
			ctxlog.Infof(ctx, "Error generating SSH key secret: %s", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating SSH key secret failed.")
		}
	case qsv1a1.Certificate:
		ctxlog.Info(ctx, "Generating certificate")
		err = r.createCertificateSecret(ctx, instance)
		if err != nil {
			if isCaNotReady(err) {
				ctxlog.Info(ctx, fmt.Sprintf("CA for secret '%s' is not ready yet: %s", instance.Name, err))
				return reconcile.Result{RequeueAfter: time.Second * 5}, nil
			}
			ctxlog.Info(ctx, "Error generating certificate secret: "+err.Error())
			return reconcile.Result{}, errors.Wrap(err, "generating certificate secret.")
		}
	default:
		err = ctxlog.WithEvent(instance, "InvalidTypeError").Errorf(ctx, "Invalid type: %s", instance.Spec.Type)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.updateQSecret(ctx, instance)
}

func (r *ReconcileQuarksSecret) updateQSecret(ctx context.Context, instance *qsv1a1.QuarksSecret) error {
	instance.Status.Generated = true
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, instance, mutate.QuarksSecretMutateFn(instance))
	if err != nil {
		return errors.Wrapf(err, "could not create or update QuarksSecret '%s'", instance.GetName())
	}

	ctxlog.Debugf(ctx, "QuarksSecret '%s' has been %s", instance.Name, op)

	now := metav1.Now()
	instance.Status.LastReconcile = &now
	err = r.client.Status().Update(ctx, instance)
	if err != nil {
		return errors.Wrapf(err, "could not create or update QuarksSecret status '%s'", instance.GetName())
	}

	return nil
}

func (r *ReconcileQuarksSecret) createPasswordSecret(ctx context.Context, instance *qsv1a1.QuarksSecret) error {
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

func (r *ReconcileQuarksSecret) createRSASecret(ctx context.Context, instance *qsv1a1.QuarksSecret) error {
	key, err := r.generator.GenerateRSAKey(instance.GetName())
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.SecretName,
			Namespace: instance.GetNamespace(),
		},
		StringData: map[string]string{
			"private_key": string(key.PrivateKey),
			"public_key":  string(key.PublicKey),
		},
	}

	return r.createSecret(ctx, instance, secret)
}

func (r *ReconcileQuarksSecret) createSSHSecret(ctx context.Context, instance *qsv1a1.QuarksSecret) error {
	key, err := r.generator.GenerateSSHKey(instance.GetName())
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.SecretName,
			Namespace: instance.GetNamespace(),
		},
		StringData: map[string]string{
			"private_key":            string(key.PrivateKey),
			"public_key":             string(key.PublicKey),
			"public_key_fingerprint": key.Fingerprint,
		},
	}

	return r.createSecret(ctx, instance, secret)
}

func (r *ReconcileQuarksSecret) createCertificateSecret(ctx context.Context, instance *qsv1a1.QuarksSecret) error {

	serviceIPForEKSWorkaround := ""

	for _, serviceRef := range instance.Spec.Request.CertificateRequest.ServiceRef {
		service := &corev1.Service{}

		err := r.client.Get(ctx, types.NamespacedName{Namespace: r.config.Namespace, Name: serviceRef.Name}, service)

		if err != nil {
			return errors.Wrapf(err, "Failed to get service reference '%s' for QuarksSecret '%s'", serviceRef.Name, instance.Name)
		}

		if serviceIPForEKSWorkaround == "" {
			serviceIPForEKSWorkaround = service.Spec.ClusterIP
		}

		instance.Spec.Request.CertificateRequest.AlternativeNames = append(append(
			instance.Spec.Request.CertificateRequest.AlternativeNames,
			service.Name,
			service.Name+"."+service.Namespace,
			"*."+service.Name,
			"*."+service.Name+"."+service.Namespace,
			service.Spec.ClusterIP,
			service.Spec.LoadBalancerIP,
			service.Spec.ExternalName,
		), service.Spec.ExternalIPs...)
	}

	if len(instance.Spec.Request.CertificateRequest.SignerType) == 0 {
		instance.Spec.Request.CertificateRequest.SignerType = qsv1a1.LocalSigner
	}

	generationRequest, err := r.generateCertificateGenerationRequest(ctx, instance.Namespace, instance.Spec.Request.CertificateRequest)
	if err != nil {
		return errors.Wrap(err, "generating certificate generation request")
	}

	switch instance.Spec.Request.CertificateRequest.SignerType {
	case qsv1a1.ClusterSigner:
		if instance.Spec.Request.CertificateRequest.ActivateEKSWorkaroundForSAN {
			if serviceIPForEKSWorkaround == "" {
				return errors.Errorf("can't activate EKS workaround for QuarksSecret '%s/%s'; couldn't find a ClusterIP for any service reference", instance.Namespace, instance.Name)
			}

			ctxlog.Infof(ctx, "Activating EKS workaround for QuarksSecret '%s/%s'. Using IP '%s' as a common name. See 'https://github.com/awslabs/amazon-eks-ami/issues/341' for more details.", instance.Namespace, instance.Name, serviceIPForEKSWorkaround)

			generationRequest.CommonName = serviceIPForEKSWorkaround
		}

		ctxlog.Info(ctx, "Generating certificate signing request and its key")
		csr, key, err := r.generator.GenerateCertificateSigningRequest(generationRequest)
		if err != nil {
			return err
		}

		// private key Secret which will be merged to certificate Secret later
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      names.CsrPrivateKeySecretName(names.CSRName(instance.Namespace, instance.Name)),
				Namespace: instance.GetNamespace(),
			},
			StringData: map[string]string{
				"private_key": string(key),
				"is_ca":       strconv.FormatBool(instance.Spec.Request.CertificateRequest.IsCA),
			},
		}

		err = r.createSecret(ctx, instance, secret)
		if err != nil {
			return err
		}

		return r.createCertificateSigningRequest(ctx, instance, csr)
	case qsv1a1.LocalSigner:
		// Generate certificate
		cert, err := r.generator.GenerateCertificate(instance.GetName(), generationRequest)
		if err != nil {
			return err
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Spec.SecretName,
				Namespace: instance.GetNamespace(),
			},
			StringData: map[string]string{
				"certificate": string(cert.Certificate),
				"private_key": string(cert.PrivateKey),
				"is_ca":       strconv.FormatBool(instance.Spec.Request.CertificateRequest.IsCA),
			},
		}

		if len(generationRequest.CA.Certificate) > 0 {
			secret.StringData["ca"] = string(generationRequest.CA.Certificate)
		}

		return r.createSecret(ctx, instance, secret)
	default:
		return fmt.Errorf("unrecognized signer type: %s", instance.Spec.Request.CertificateRequest.SignerType)
	}
}

func (r *ReconcileQuarksSecret) canBeGenerated(ctx context.Context, instance *qsv1a1.QuarksSecret) (bool, error) {
	// Skip secret generation when instance has generated secret and its `generated` status is true
	if instance.Status.Generated {
		return false, nil
	}

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

	if secretLabels[qsv1a1.LabelKind] != qsv1a1.GeneratedSecretKind {
		return false, nil
	}

	return true, nil
}

// createSecret applies common properties(labels and ownerReferences) to the secret and creates it
func (r *ReconcileQuarksSecret) createSecret(ctx context.Context, instance *qsv1a1.QuarksSecret, secret *corev1.Secret) error {
	ctxlog.Debugf(ctx, "Creating secret '%s'", secret.Name)

	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		secretLabels = map[string]string{}
	}

	secretLabels[qsv1a1.LabelKind] = qsv1a1.GeneratedSecretKind

	secret.SetLabels(secretLabels)

	if err := r.setReference(instance, secret, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for secret '%s' to QuarksSecret '%s' in namespace '%s'", secret.GetName(), instance.GetName(), instance.GetNamespace())
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.client, secret, mutate.SecretMutateFn(secret))
	if err != nil {
		return errors.Wrapf(err, "could not create or update secret '%s'", secret.GetName())
	}

	ctxlog.Debugf(ctx, "Secret '%s' has been %s", secret.Name, op)

	return nil
}

// generateCertificateGenerationRequest generates CertificateGenerationRequest for certificate
func (r *ReconcileQuarksSecret) generateCertificateGenerationRequest(ctx context.Context, namespace string, certificateRequest qsv1a1.CertificateRequest) (credsgen.CertificateGenerationRequest, error) {
	var request credsgen.CertificateGenerationRequest
	switch certificateRequest.SignerType {
	case qsv1a1.ClusterSigner:
		// Generate cluster-signed CA certificate
		request = credsgen.CertificateGenerationRequest{
			CommonName:       certificateRequest.CommonName,
			AlternativeNames: certificateRequest.AlternativeNames,
		}
	case qsv1a1.LocalSigner:
		// Generate local-issued CA certificate
		request = credsgen.CertificateGenerationRequest{
			IsCA:             certificateRequest.IsCA,
			CommonName:       certificateRequest.CommonName,
			AlternativeNames: certificateRequest.AlternativeNames,
		}

		if len(certificateRequest.CARef.Name) > 0 {
			// Get CA certificate
			caSecret := &corev1.Secret{}
			caNamespacedName := types.NamespacedName{
				Namespace: namespace,
				Name:      certificateRequest.CARef.Name,
			}
			err := r.client.Get(ctx, caNamespacedName, caSecret)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return request, newCaNotReadyError("CA secret not found")
				}
				return request, errors.Wrap(err, "getting CA secret")
			}
			ca := caSecret.Data[certificateRequest.CARef.Key]

			// Get CA key
			if certificateRequest.CAKeyRef.Name != certificateRequest.CARef.Name {
				caSecret = &corev1.Secret{}
				caNamespacedName = types.NamespacedName{
					Namespace: namespace,
					Name:      certificateRequest.CAKeyRef.Name,
				}
				err = r.client.Get(ctx, caNamespacedName, caSecret)
				if err != nil {
					if apierrors.IsNotFound(err) {
						return request, newCaNotReadyError("CA key secret not found")
					}
					return request, errors.Wrap(err, "getting CA Key secret")
				}
			}
			key := caSecret.Data[certificateRequest.CAKeyRef.Key]
			request.CA = credsgen.Certificate{
				IsCA:        true,
				PrivateKey:  key,
				Certificate: ca,
			}
		}
	default:
		return request, fmt.Errorf("unrecognized signer type: %s", certificateRequest.SignerType)
	}

	return request, nil
}

// createCertificateSigningRequest creates CertificateSigningRequest Object
func (r *ReconcileQuarksSecret) createCertificateSigningRequest(ctx context.Context, instance *qsv1a1.QuarksSecret, csr []byte) error {
	csrName := names.CSRName(instance.Namespace, instance.Name)
	ctxlog.Debugf(ctx, "Creating certificateSigningRequest '%s'", csrName)

	annotations := instance.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[qsv1a1.AnnotationCertSecretName] = instance.Spec.SecretName
	annotations[qsv1a1.AnnotationQSecNamespace] = instance.Namespace

	csrObj := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        csrName,
			Labels:      instance.Labels,
			Annotations: annotations,
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Request: csr,
			Usages:  instance.Spec.Request.CertificateRequest.Usages,
		},
	}

	if err := r.setReference(instance, csrObj, r.scheme); err != nil {
		return errors.Wrapf(err, "error setting owner for certificateSigningRequest '%s' to QuarksSecret '%s' in namespace '%s'", csrObj.Name, instance.Name, instance.GetNamespace())
	}

	// CSR spec is immutable after the request is created
	err := r.client.Get(ctx, types.NamespacedName{Name: csrObj.Name}, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = r.client.Create(ctx, csrObj)
			if err != nil {
				return errors.Wrapf(err, "could not create certificateSigningRequest '%s'", csrObj.Name)
			}
			return nil
		}
		return errors.Wrapf(err, "could not get certificateSigningRequest '%s'", csrObj.Name)
	}

	return nil
}
