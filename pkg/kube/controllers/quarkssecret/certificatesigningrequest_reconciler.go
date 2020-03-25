package quarkssecret

import (
	"context"
	"fmt"

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

	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
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
	csr := &certv1.CertificateSigningRequest{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling CSR '%s'", request.Name)
	err := r.client.Get(ctx, request.NamespacedName, csr)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// Return and don't requeue
			ctxlog.Info(ctx, "Skip reconcile: CSR not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, errors.Wrap(err, "Error reading CSR")
	}

	if len(csr.Status.Certificate) != 0 {
		ctxlog.Debugf(ctx, "CSR '%s' is in 'issued' state", csr.Name)

		// Validate annotations on CSR, should be correct, since it created by the quarks secret reconciler
		annotations := csr.GetAnnotations()
		if annotations == nil {
			ctxlog.WithEvent(csr, "NotFoundError").Errorf(ctx, "Empty annotations of CSR '%s'", csr.Name)
			return reconcile.Result{}, nil
		}
		secretName, ok := annotations[qsv1a1.AnnotationCertSecretName]
		if !ok {
			ctxlog.WithEvent(csr, "NotFoundError").Errorf(ctx, "Failed to lookup cert secret name from CSR '%s' annotations", csr.Name)
			return reconcile.Result{}, nil
		}
		namespace, ok := annotations[qsv1a1.AnnotationQSecNamespace]
		if !ok {
			ctxlog.WithEvent(csr, "NotFoundError").Errorf(ctx, "Failed to lookup quarksSecret namespace from CSR '%s' annotations", csr.Name)
			return reconcile.Result{}, nil
		}
		qsecName, ok := annotations[qsv1a1.AnnotationQSecName]
		if !ok {
			ctxlog.WithEvent(csr, "NotFoundError").Errorf(ctx, "Failed to lookup quarksSecret name from CSR '%s' annotations", csr.Name)
			return reconcile.Result{}, nil
		}

		// Wait for CSR to result in a private key
		privateKeySecret := &corev1.Secret{}
		keySecretName := names.CsrPrivateKeySecretName(csr.Name)
		err := r.client.Get(ctx, types.NamespacedName{Name: keySecretName, Namespace: namespace}, privateKeySecret)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Waiting for CSR private key secret '%s'", keySecretName)
				return reconcile.Result{Requeue: true}, nil
			}
			ctxlog.Errorf(ctx, "Failed to get the CSR private key secret: %v", err.Error())
			return reconcile.Result{}, err
		}

		// Load quarks secret which created this CSR, so we can set it as an owner of the resulting
		// certificate secret
		qsec := &qsv1a1.QuarksSecret{}
		err = r.client.Get(ctx, types.NamespacedName{Name: qsecName, Namespace: namespace}, qsec)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to get the quarks secret '%s' for this CSR:  %v", qsecName, err.Error())
			return reconcile.Result{}, err
		}

		// Create the certificate secret
		rootCA, err := getClusterRootCA(ctx, r.client, namespace)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to get the cluster root CA: %v", err.Error())
			return reconcile.Result{}, nil
		}

		certSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels: map[string]string{
					qsv1a1.LabelKind: qsv1a1.GeneratedSecretKind,
				},
			},
			Data: map[string][]byte{
				"ca":          rootCA,
				"certificate": csr.Status.Certificate,
				"private_key": privateKeySecret.Data["private_key"],
				"is_ca":       privateKeySecret.Data["is_ca"],
			},
		}
		r.setReference(qsec, certSecret, r.scheme)

		ctxlog.Infof(ctx, "Creating certificate secret '%s' for CSR '%s'", certSecret.Name, csr.Name)
		err = r.createSecret(ctx, certSecret)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to create the approved certificate secret: %v", err.Error())
			return reconcile.Result{}, err
		}

		// Clean up CSR and private key, no longer needed
		err = r.deleteSecret(ctx, privateKeySecret)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to delete the CSR private key secret: %v", err.Error())
			return reconcile.Result{}, err
		}

		err = r.deleteCSR(ctx, csr)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to remove completed CSR: %v", err.Error())
			return reconcile.Result{}, err
		}

	} else {
		err = r.approveRequest(ctx, csr.Name)
		if err != nil {
			ctxlog.Errorf(ctx, "Failed to approve certificate signing request: %v", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "approving cert request failed")
		}
	}

	return reconcile.Result{}, nil
}

// approveRequest approves the CSR
func (r *ReconcileCertificateSigningRequest) approveRequest(ctx context.Context, csrName string) error {
	csr, err := r.certClient.CertificateSigningRequests().Get(csrName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "could not get CSR '%s'", csrName)
	}

	if isApproved(csr.Status.Conditions) {
		ctxlog.Debugf(ctx, "Waiting for CSR to be issued: CSR %s has already been approved", csrName)
		return nil
	}

	csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Type:    certv1.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "This CSR was approved by csr-controller",
	})

	ctxlog.Infof(ctx, "Approving CSR '%s'", csrName)
	_, err = r.certClient.CertificateSigningRequests().UpdateApproval(csr)
	if err != nil {
		return errors.Wrapf(err, "could not update approval of CSR '%s'", csrName)
	}

	ctxlog.Debugf(ctx, "CSR '%s' has been updated", csrName)

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

func (r *ReconcileCertificateSigningRequest) deleteSecret(ctx context.Context, secret *corev1.Secret) error {
	ctxlog.Debugf(ctx, "Deleting secret '%s'", secret.Name)

	err := r.client.Delete(ctx, secret)
	if err != nil {
		return errors.Wrapf(err, "could not delete secret '%s/%s'", secret.Namespace, secret.Name)
	}

	return nil
}

func (r *ReconcileCertificateSigningRequest) deleteCSR(ctx context.Context, csr *certv1.CertificateSigningRequest) error {
	ctxlog.Debugf(ctx, "Deleting csr '%s'", csr.Name)

	err := r.client.Delete(ctx, csr)
	if err != nil {
		return errors.Wrapf(err, "could not delete csr '%s'", csr.Name)
	}

	return nil
}

// isApproved returns true if the CSR has already been approved
func isApproved(conditions []certv1.CertificateSigningRequestCondition) bool {
	for _, condition := range conditions {
		if condition.Type == certv1.CertificateApproved {
			return true
		}
	}

	return false
}

func getClusterRootCA(ctx context.Context, c client.Client, namespace string) ([]byte, error) {
	// TODO: This should work with filtering using something like
	// err = client.List(ctx, secretList, client.InNamespace(namespace), client.MatchingFields{"type": "kubernetes.io/service-account-token"})
	// but it doesn't return any results. Maybe try again after a controller-runtime bump

	secretList := &corev1.SecretList{}
	err := c.List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "Could not get the list of secrets")
	}

	if len(secretList.Items) <= 0 {
		return nil, fmt.Errorf("failed to get a service account token secret to extract the cluster root CA")
	}

	for _, secret := range secretList.Items {
		if secret.Type == "kubernetes.io/service-account-token" {
			if val, ok := secret.Data["ca.crt"]; ok {
				return val, nil
			}

			return nil, fmt.Errorf("the service account token secret does not contain the cluster root CA")
		}
	}

	return nil, fmt.Errorf("could not find a service account token secret")
}
