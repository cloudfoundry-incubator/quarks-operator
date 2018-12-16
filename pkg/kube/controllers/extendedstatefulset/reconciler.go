package extendedstatefulset

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	essv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Check that ReconcileExtendedStatefulSet implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileExtendedStatefulSet{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(log *zap.SugaredLogger, mgr manager.Manager, srf setReferenceFunc) reconcile.Reconciler {
	log.Info("Creating a reconciler for ExtendedStatefulSet")

	return &ReconcileExtendedStatefulSet{
		log:          log,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		setReference: srf,
	}
}

// ReconcileExtendedStatefulSet reconciles an ExtendedStatefulSet object
type ReconcileExtendedStatefulSet struct {
	client       client.Client
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	log          *zap.SugaredLogger
}

// Reconcile reads that state of the cluster for a ExtendedStatefulSet object
// and makes changes based on the state read and what is in the ExtendedStatefulSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileExtendedStatefulSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Info("Reconciling ExtendedStatefulSet ", request.NamespacedName)

	// Fetch the ExtendedStatefulSet we need to reconcile
	exStatefulSet := &essv1a1.ExtendedStatefulSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, exStatefulSet)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Debug("Skip reconcile: ExtendedStatefulSet not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// TODO: generate an ID for the request

	// Get the actual StatefulSet
	actualStatefulSet, actualVersion, err := r.getActualStatefulSet(context.TODO(), exStatefulSet)
	if err != nil {
		r.log.Error("Could not retrieve latest StatefulSet for ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	// Calculate the desired StatefulSet
	desiredStatefulSet, desiredVersion, err := r.calculateDesiredStatefulSet(exStatefulSet, actualStatefulSet)
	if err != nil {
		r.log.Error("Could not calculate StatefulSet for ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	// If actual version is zero, there is no StatefulSet live
	if actualVersion != desiredVersion {
		// If it doesn't exist, create it
		r.log.Info("StatefulSet '", desiredStatefulSet.Name, "' for ExtendedStatefulSet '", request.NamespacedName, "' not found, will be created.")

		if err := r.createStatefulSet(context.TODO(), exStatefulSet, desiredStatefulSet); err != nil {
			r.log.Error("Could not create StatefulSet for ExtendedStatefulSet '", request.NamespacedName, "': ", err)
			return reconcile.Result{}, err
		}
	} else {
		// If it does exist, do a deep equal and check that we own it
		r.log.Info("StatefulSet '", desiredStatefulSet.Name, "' for ExtendedStatefulSet '", request.NamespacedName, "' has not changed, checking if any other changes are necessary.")

		// 	if !metav1.IsControlledBy(deployment, foo) {

		// If equal, we don't need to do anything
		// if not equal, see what's different and act accordingly
	}

	// Find a way to check result
	statefulSetVersions, err := r.listStatefulSetVersions(context.TODO(), exStatefulSet)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	r.log.Info(statefulSetVersions)

	// Cleanup
	err = r.cleanupStatefulSets(context.TODO(), exStatefulSet)
	if err != nil {
		r.log.Error("Could not cleanup StatefulSets for ExtendedStatefulSet '", request.NamespacedName, "': ", err)

		return reconcile.Result{}, err
	}

	//statefulSet.GenerateName
	//statefulSet.GetOwnerReferences
	// Which should be cleaned up?

	// Update the Status of the resource
	// Update status.Versions if needed
	if !reflect.DeepEqual(statefulSetVersions, exStatefulSet.Status.Versions) {
		exStatefulSet.Status.Versions = statefulSetVersions
		err := r.client.Update(context.TODO(), exStatefulSet)
		if err != nil {
			r.log.Error("Failed to update exStatefulSet status: %v\n", err)
			return reconcile.Result{}, err
		}
	}

	if !statefulSetVersions[desiredVersion] {
		r.log.Debug("Waiting desired version available")
		return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	return reconcile.Result{}, nil
}

// calculateDesiredStatefulSet generates the desired StatefulSet that should exist
func (r *ReconcileExtendedStatefulSet) calculateDesiredStatefulSet(exStatefulSet *essv1a1.ExtendedStatefulSet, actualStatefulSet *v1beta1.StatefulSet) (*v1beta1.StatefulSet, int, error) {
	result := exStatefulSet.Spec.Template

	// Place the StatefulSet in the same namespace as the ExtendedStatefulSet
	result.SetNamespace(exStatefulSet.Namespace)

	// Calculate its name
	name, err := exStatefulSet.CalculateDesiredStatefulSetName(actualStatefulSet)
	if err != nil {
		return nil, 0, err
	}
	result.SetName(name)

	// Set version and sha
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	version, err := exStatefulSet.DesiredVersion(actualStatefulSet)
	if err != nil {
		return nil, 0, err
	}
	sha, err := exStatefulSet.CalculateStatefulSetSHA1()
	if err != nil {
		return nil, 0, err
	}
	result.Annotations[essv1a1.AnnotationStatefulSetSHA1] = sha
	result.Annotations[essv1a1.AnnotationVersion] = fmt.Sprintf("%d", version)

	return &result, version, nil
}

// createStatefulSet creates a StatefulSet
func (r *ReconcileExtendedStatefulSet) createStatefulSet(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet, statefulSet *v1beta1.StatefulSet) error {

	// Set the owner of the StatefulSet, so it's garbage collected,
	// and we can find it later
	r.log.Info("Setting owner for StatefulSet '", statefulSet.Name, "' to ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")
	if err := r.setReference(exStatefulSet, statefulSet, r.scheme); err != nil {
		return errors.Wrapf(err, "Could not set owner for StatefulSet '%s' to ExtendedStatefulSet '%s' in namespace '%s'", statefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
	}

	// Create the StatefulSet
	if err := r.client.Create(ctx, statefulSet); err != nil {
		return errors.Wrapf(err, "Could not create StatefulSet '%s' for ExtendedStatefulSet '%s' in namespace '%s'", statefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
	}

	r.log.Info("Created StatefulSet '", statefulSet.Name, "' for ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")

	return nil
}

// cleanupStatefulSets cleans up StatefulSets if they are no longer required
func (r *ReconcileExtendedStatefulSet) cleanupStatefulSets(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet) error {
	r.log.Info("Cleaning up StatefulSets for ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "Couldn't list StatefulSets for cleanup")
	}

	for _, statefulSet := range statefulSets {
		r.log.Debug("Considering StatefulSet '", statefulSet.Name, "' for cleanup.")
	}

	return nil
}

// listStatefulSets gets all StatefulSets owned by the ExtendedStatefulSet
func (r *ReconcileExtendedStatefulSet) listStatefulSets(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet) ([]v1beta1.StatefulSet, error) {
	r.log.Debug("Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	result := []v1beta1.StatefulSet{}

	// Get owned resources
	// Go through each StatefulSet
	allStatefulSets := &v1beta1.StatefulSetList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     exStatefulSet.Namespace,
			LabelSelector: labels.Everything(),
		},
		allStatefulSets)
	if err != nil {
		return nil, err
	}

	for _, statefulSet := range allStatefulSets.Items {
		if metav1.IsControlledBy(&statefulSet, exStatefulSet) {
			result = append(result, statefulSet)
			r.log.Debug("StatefulSet '", statefulSet.Name, "' owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")
		} else {
			r.log.Debug("StatefulSet '", statefulSet.Name, "' is not owned by ExtendedStatefulSet '", exStatefulSet.Name, "', ignoring.")
		}
	}

	return result, nil
}

// getActualStatefulSet gets the latest (by version) StatefulSet owned by the ExtendedStatefulSet
func (r *ReconcileExtendedStatefulSet) getActualStatefulSet(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet) (*v1beta1.StatefulSet, int, error) {
	r.log.Debug("Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	// Default response is an empty StatefulSet with version '0' and an empty signature
	result := &v1beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				essv1a1.AnnotationStatefulSetSHA1: "",
				essv1a1.AnnotationVersion:         "0",
			},
		},
	}
	maxVersion := 0

	// Get all owned StatefulSets
	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return nil, 0, err
	}

	for _, ss := range statefulSets {
		strVersion := ss.Annotations[essv1a1.AnnotationVersion]
		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return nil, 0, err
		}

		if ss.Annotations != nil && version > maxVersion {
			result = &ss
			maxVersion = version
		}
	}

	return result, maxVersion, nil
}

// listStatefulSetVersions gets all StatefulSets' versions and ready status owned by the ExtendedStatefulSet
func (r *ReconcileExtendedStatefulSet) listStatefulSetVersions(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet) (map[int]bool, error) {
	result := map[int]bool{}

	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return nil, err
	}

	for _, statefulSet := range statefulSets {
		strVersion, found := statefulSet.Annotations[essv1a1.AnnotationVersion]
		if !found {
			return result, errors.Errorf("Version annotation is not found from: %+v", statefulSet.Annotations)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return result, errors.Wrapf(err, "Version annotation is not an int: %s", strVersion)
		}

		ready, err := r.isStatefulSetReady(ctx, &statefulSet)
		if err != nil {
			return nil, err
		}

		if ready {
			result[version] = true
		} else {
			result[version] = false
		}
	}

	return result, nil
}

// isStatefulSetReady returns true if one owned Pod is running
func (r *ReconcileExtendedStatefulSet) isStatefulSetReady(ctx context.Context, statefulSet *v1beta1.StatefulSet) (bool, error) {
	podList := &v1.PodList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     statefulSet.Namespace,
			LabelSelector: labels.Everything(),
		},
		podList,
	)
	if err != nil {
		return false, err
	}

	for _, pod := range podList.Items {
		if metav1.IsControlledBy(&pod, statefulSet) {
			if isPodReady(&pod) {
				r.log.Debug("Pod '", statefulSet.Name, "' owned by StatefulSet '", statefulSet.Name, "' is running.")
				return true, nil
			}
		}
	}

	return false, nil
}

// isPodReady returns false if the Pod Status is nil
func isPodReady(pod *v1.Pod) bool {
	var condition *v1.PodCondition
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == v1.PodReady {
			condition = &pod.Status.Conditions[i]
			break
		}
	}
	return condition != nil && condition.Status == v1.ConditionTrue
}
