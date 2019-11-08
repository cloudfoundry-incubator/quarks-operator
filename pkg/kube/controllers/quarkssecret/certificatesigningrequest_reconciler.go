package quarkssecret

import (
	"context"

	"github.com/pkg/errors"
	certv1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	certv1client "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qev1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// NewCertificateSigningRequestReconciler returns a new Reconciler
func NewCertificateSigningRequestReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, certClient certv1client.CertificatesV1beta1Interface, srf setReferenceFunc) reconcile.Reconciler {
	return &ReconcileCertificateSigningRequest{
		ctx:          ctx,
		config:       config,
		client:       mgr.GetClient(),
		certClient:   certClient,
		scheme:       mgr.GetScheme(),
		setReference: srf,
	}
}

// ReconcileCertificateSigningRequest reconciles an CertificateSigningRequest object
type ReconcileCertificateSigningRequest struct {
	ctx          context.Context
	config       *config.Config
	client       client.Client
	certClient   certv1client.CertificatesV1beta1Interface
	scheme       *runtime.Scheme
	setReference setReferenceFunc
}

// Reconcile approves pending CSR and creates its certificate secret
func (r *ReconcileCertificateSigningRequest) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &certv1.CertificateSigningRequest{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling certificateSigningRequest '%s'", request.Name)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Info(ctx, "Skip reconcile: certificateSigningRequest not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, errors.Wrap(err, "Error reading certificateSigningRequest")
	}

	if len(instance.Status.Certificate) != 0 {
		annotations := instance.GetAnnotations()
		if annotations == nil {
			ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "Empty annotations of certificateSigningRequest '%s'", instance.Name)
			return reconcile.Result{}, nil
		}
		secretName, ok := annotations[qev1a1.AnnotationCertSecretName]
		if !ok {
			ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "failed to lookup cert secret name from certificateSigningRequest '%s' annotations", instance.Name)
			return reconcile.Result{}, nil
		}
		namespace, ok := annotations[qev1a1.AnnotationQSecNamespace]
		if !ok {
			ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "failed to lookup exSecret namespace from certificateSigningRequest '%s' annotations", instance.Name)
			return reconcile.Result{}, nil
		}

		privatekeySecret, err := r.getSecret(ctx, namespace, names.CsrPrivateKeySecretName(instance.Name))
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to get the CSR private key secret: %v", err.Error())
			return reconcile.Result{}, err
		}
		certSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secretName,
				Namespace:       namespace,
				OwnerReferences: instance.OwnerReferences,
				Labels: map[string]string{
					qev1a1.LabelKind: qev1a1.GeneratedSecretKind,
				},
			},
			Data: map[string][]byte{
				"certificate": instance.Status.Certificate,
				"private_key": privatekeySecret.Data["private_key"],
				"is_ca":       privatekeySecret.Data["is_ca"],
			},
		}

		err = r.createSecret(ctx, certSecret)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to create the approved certificate secret: %v", err.Error())
			return reconcile.Result{}, err
		}

		err = r.deleteSecret(ctx, privatekeySecret)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to delete the CSR private key secret: %v", err.Error())
			return reconcile.Result{}, err
		}
	} else {
		err = r.approveRequest(ctx, instance.Name)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to approve certificate signing request: %v", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "approving cert request failed")
		}
	}

	return reconcile.Result{}, nil
}

// approveRequest approves the certificateSigningRequest
func (r *ReconcileCertificateSigningRequest) approveRequest(ctx context.Context, csrName string) error {
	csr, err := r.certClient.CertificateSigningRequests().Get(csrName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "could not get certificateSigningRequest '%s'", csrName)
	}

	if isApproved(csr.Status.Conditions) {
		ctxlog.Debugf(ctx, "Skip approve: certificateSigningRequest %s has already been approved", csrName)
		return nil
	}

	ctxlog.Debugf(ctx, "Approving certificateSigningRequest '%s'", csrName)

	csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Type:    certv1.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "This CSR was approved by csr-controller",
	})

	_, err = r.certClient.CertificateSigningRequests().UpdateApproval(csr)
	if err != nil {
		return errors.Wrapf(err, "could not update approval of certificateSigningRequest '%s'", csrName)
	}

	ctxlog.Debugf(ctx, "CertificateSigningRequest '%s' has been updated", csrName)

	return nil
}

// createSecret creates secret
func (r *ReconcileCertificateSigningRequest) createSecret(ctx context.Context, secret *corev1.Secret) error {
	ctxlog.Debugf(ctx, "Creating secret '%s'", secret.Name)

	obj := secret.DeepCopy()
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, obj, func() error {
		obj.StringData = secret.StringData
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "could not create or update secret '%s'", secret.GetName())
	}

	ctxlog.Debugf(ctx, "Secret '%s' has been %s", secret.Name, op)

	return nil
}

// getSecret gets secret
func (r *ReconcileCertificateSigningRequest) getSecret(ctx context.Context, namespace string, secretName string) (*corev1.Secret, error) {
	ctxlog.Debugf(ctx, "getting secret '%s'", secretName)

	secret := &corev1.Secret{}
	err := r.client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
	if err != nil {
		return secret, errors.Wrapf(err, "could not get secret '%s/%s'", namespace, secretName)
	}
	return secret, nil
}

// deleteSecret deletes secret
func (r *ReconcileCertificateSigningRequest) deleteSecret(ctx context.Context, secret *corev1.Secret) error {
	ctxlog.Debugf(ctx, "Deleting secret '%s'", secret.Name)

	err := r.client.Delete(ctx, secret)
	if err != nil {
		return errors.Wrapf(err, "could not delete secret '%s/%s'", secret.Namespace, secret.Name)
	}

	return nil
}

// isApproved returns true if the certificateSigningRequest has already been approved
func isApproved(conditions []certv1.CertificateSigningRequestCondition) bool {
	for _, condition := range conditions {
		if condition.Type == certv1.CertificateApproved {
			return true
		}
	}

	return false
}
