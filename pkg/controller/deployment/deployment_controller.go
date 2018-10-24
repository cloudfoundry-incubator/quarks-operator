package deployment

import (
	"context"
	"log"

	fissilev1alpha1 "github.com/scf/cf-operator/pkg/apis/fissile/v1alpha1"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Deployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Deployment
	err = c.Watch(&source.Kind{Type: &fissilev1alpha1.Deployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Deployment
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &fissilev1alpha1.Deployment{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDeployment{}

// ReconcileDeployment reconciles a Deployment object
type ReconcileDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Deployment object and makes changes based on the state read
// and what is in the Deployment.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
type InstanceGroup struct {
	Name      string
	Instances int
}
type Manifest struct {
	InstanceGroups []InstanceGroup `yaml:"instance-groups"`
}

//(m Manifest) GetObjectKind() schema.ObjectKind { return nil }

func (r *ReconcileDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Reconciling Deployment %s/%s\n", request.Namespace, request.Name)

	// Fetch the Deployment instance
	instance := &fissilev1alpha1.Deployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// retrieve secret manifest
	config := &corev1.ConfigMap{}
	ref := instance.Spec.ManifestRef
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ref}, config)
	if err != nil {
		return reconcile.Result{}, err
	}
	// unmarshal manifet.data into bosh deployment mana... LoadManifest() from fisisle
	m, ok := config.Data["manifest"]
	if !ok {
		return reconcile.Result{}, nil
	}
	manifest := &Manifest{}
	err = yaml.Unmarshal([]byte(m), manifest)
	if err != nil {
		return reconcile.Result{}, nil
	}

	// TODO validation

	// Define a new Pod object
	pod := newPodForCR(manifest)

	// Set Deployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating a new Pod %s/%s\n", pod.Namespace, pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	log.Printf("Skip reconcile: Pod %s/%s already exists", found.Namespace, found.Name)
	return reconcile.Result{}, nil
}

// TODO:
// *  alidate manifest
// *

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *Manifest) *corev1.Pod {
	ig := cr.InstanceGroups[0]
	labels := map[string]string{
		"app": ig.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ig.Name + "-pod",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}
