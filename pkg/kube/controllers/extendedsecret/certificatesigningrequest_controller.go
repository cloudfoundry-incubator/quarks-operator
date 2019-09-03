package extendedsecret

import (
	"context"

	"github.com/pkg/errors"
	certv1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certv1client "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// AddCertificateSigningRequest creates a new CertificateSigningRequest controller to watch for new and changed
// certificate signing request. Reconciliation will approve them and create a secret.
func AddCertificateSigningRequest(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "csr-reconciler", mgr.GetEventRecorderFor("csr-recorder"))
	certClient, err := certv1client.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}
	r := NewCertificateSigningRequestReconciler(ctx, config, mgr, certClient, controllerutil.SetControllerReference)

	// Create a new controller
	c, err := controller.New("certificatesigningrequest-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxExtendedSecretWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding certificate signing request controller to manager failed.")
	}

	// Watch for changes to CertificateSigningRequests
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*certv1.CertificateSigningRequest)

			return ownedByExSecret(o.OwnerReferences)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectNew.(*certv1.CertificateSigningRequest)

			return ownedByExSecret(o.OwnerReferences)

		},
	}
	err = c.Watch(&source.Kind{Type: &certv1.CertificateSigningRequest{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return errors.Wrapf(err, "Watching extended secrets failed in certificate signing request controller.")
	}

	return nil
}

func ownedByExSecret(owners []metav1.OwnerReference) bool {
	for _, owner := range owners {
		if owner.Kind == "ExtendedSecret" {
			return true
		}
	}
	return false
}
