package extendedsecret

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	credsgen "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	es "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// Add creates a new ExtendedSecrets Controller and adds it to the Manager
func Add(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-secret-reconciler", mgr.GetRecorder("ext-secret-recorder"))
	log := ctxlog.ExtractLogger(ctx)
	r := NewReconciler(ctx, config, mgr, credsgen.NewInMemoryGenerator(log), controllerutil.SetControllerReference)

	// Create a new controller
	c, err := controller.New("extendedsecret-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ExtendedSecrets
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*es.ExtendedSecret)
			secrets, err := listSecrets(ctx, mgr.GetClient(), o)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to list StatefulSets owned by ExtendedStatefulSet '%s': %s", o.Name, err)
			}

			return len(secrets) == 0
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
	}
	err = c.Watch(&source.Kind{Type: &es.ExtendedSecret{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return nil
}

// listSecrets gets all Secrets owned by the ExtendedSecret
func listSecrets(ctx context.Context, client crc.Client, exSecret *es.ExtendedSecret) ([]corev1.Secret, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exSecret.Name, "'.")

	secretLabelsSet := labels.Set{
		esv1.LabelKind: esv1.GeneratedSecretKind,
	}

	result := []corev1.Secret{}

	allSecrets := &corev1.SecretList{}
	err := client.List(
		ctx,
		&crc.ListOptions{
			Namespace:     exSecret.Namespace,
			LabelSelector: secretLabelsSet.AsSelector(),
		},
		allSecrets)
	if err != nil {
		return nil, err
	}

	for _, secret := range allSecrets.Items {
		if metav1.IsControlledBy(&secret, exSecret) {
			result = append(result, secret)
			ctxlog.Debug(ctx, "Secret '", secret.Name, "' owned by ExtendedSecret '", exSecret.Name, "'.")
		} else {
			ctxlog.Debug(ctx, "Secret '", secret.Name, "' is not owned by ExtendedSecret '", exSecret.Name, "', ignoring.")
		}
	}

	return result, nil
}
