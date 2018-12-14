package extendedsecret

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	esapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
)

func NewReconciler(log *zap.SugaredLogger, mgr manager.Manager, generator credsgen.Generator) reconcile.Reconciler {
	return &ReconcileExtendedSecret{
		log:       log,
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		generator: generator,
	}
}

type ReconcileExtendedSecret struct {
	client    client.Client
	generator credsgen.Generator
	scheme    *runtime.Scheme
	log       *zap.SugaredLogger
}

func (r *ReconcileExtendedSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Infof("Reconciling ExtendedSecret %s\n", request.NamespacedName)

	instance := &esapi.ExtendedSecret{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Debug("Skip reconcile: CRD not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.log.Info("Error reading the object")
		return reconcile.Result{}, err
	}

	switch instance.Spec.Type {
	case esapi.Password:
		err = r.createPasswordSecret(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "Generating password secret")
		}
	default:
		return reconcile.Result{}, fmt.Errorf("Invalid type: %s", instance.Spec.Type)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileExtendedSecret) createPasswordSecret(ctx context.Context, instance *esapi.ExtendedSecret) error {
	r.log.Debug("Generating password\n")
	request := credsgen.PasswordGenerationRequest{}
	password := r.generator.GeneratePassword("foo", request)

	// Default response is an empty StatefulSet with version '0' and an empty signature
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "es-secret-" + instance.GetName(),
			Namespace: instance.GetNamespace(),
		},
		StringData: map[string]string{
			"password": password,
		},
	}

	return r.client.Create(ctx, secret)
}
