package extendedstatefulset

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	podUtils "k8s.io/kubernetes/pkg/api/v1/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	essv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
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

	// Clean up exStatefulSet
	if exStatefulSet.ToBeDeleted() {
		r.log.Debug("ExtendedStatefulSet '", exStatefulSet.Name, "' instance marked for deletion. Clean up process.")
		return r.handleDelete(exStatefulSet)
	}

	// TODO: generate an ID for the request

	// Get the actual StatefulSet
	actualStatefulSet, actualVersion, err := r.getActualStatefulSet(context.TODO(), exStatefulSet)
	if err != nil {
		r.log.Error("Could not retrieve latest StatefulSet owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	// Calculate the desired StatefulSet
	desiredStatefulSet, desiredVersion, err := r.calculateDesiredStatefulSet(exStatefulSet, actualStatefulSet)
	if err != nil {
		r.log.Error("Could not calculate StatefulSet owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	if actualVersion == 0 {
		// Add finalizer
		r.log.Debug("Adding Finalizer to ExtendedStatefulSet '", exStatefulSet.Name, "'.")
		exStatefulSet.AddFinalizer()
	}

	// If actual version is zero, there is no StatefulSet live
	if actualVersion != desiredVersion {
		// If it doesn't exist, create it
		r.log.Info("StatefulSet '", desiredStatefulSet.Name, "' owned by ExtendedStatefulSet '", request.NamespacedName, "' not found, will be created.")

		// Record the template before creating the StatefulSet, so we don't include default values such as
		// `ImagePullPolicy`, `TerminationMessagePath`, etc. in the signature.
		originalTemplate := exStatefulSet.Spec.Template.DeepCopy()
		if err := r.createStatefulSet(context.TODO(), exStatefulSet, desiredStatefulSet); err != nil {
			r.log.Error("Could not create StatefulSet for ExtendedStatefulSet '", request.NamespacedName, "': ", err)
			return reconcile.Result{}, err
		}
		exStatefulSet.Spec.Template = *originalTemplate
	} else {
		// If it does exist, do a deep equal and check that we own it
		r.log.Info("StatefulSet '", desiredStatefulSet.Name, "' owned by ExtendedStatefulSet '", request.NamespacedName, "' has not changed, checking if any other changes are necessary.")
	}

	statefulSetVersions, err := r.listStatefulSetVersions(context.TODO(), exStatefulSet)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Update StatefulSets configSHA1 if necessary
	err = r.updateStatefulSetsConfigSHA1(context.TODO(), exStatefulSet)
	if err != nil {
		r.log.Error("Could not update StatefulSets owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	ptrStatefulSetVersions := &statefulSetVersions

	defer func() {
		// Update the Status of the resource
		if !reflect.DeepEqual(&ptrStatefulSetVersions, exStatefulSet.Status.Versions) {
			exStatefulSet.Status.Versions = statefulSetVersions
			updateErr := r.client.Update(context.TODO(), exStatefulSet)
			if updateErr != nil {
				r.log.Error("Failed to update exStatefulSet status: %v\n", updateErr)
			}
		}
	}()

	maxAvailableVersion := exStatefulSet.GetMaxAvailableVersion(statefulSetVersions)

	if len(statefulSetVersions) > 1 {
		// Cleanup versions smaller than the max available version
		err = r.cleanupStatefulSets(context.TODO(), exStatefulSet, maxAvailableVersion, &statefulSetVersions)
		if err != nil {
			r.log.Error("Could not cleanup StatefulSets owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
			return reconcile.Result{}, err
		}
	}

	if !statefulSetVersions[desiredVersion] {
		r.log.Debug("Waiting desired version available")
		return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	// Reconcile stops since only one version or no version exists.
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

	// Add Finalizer, so we can remove all its ConfigMaps and Secrets OwnerReference
	r.log.Debug("Adding Finalizer to StatefulSet '%s", statefulSet.Name, "'.")
	addFinalizer(statefulSet)

	// Create the StatefulSet
	if err := r.client.Create(ctx, statefulSet); err != nil {
		return errors.Wrapf(err, "Could not create StatefulSet '%s' for ExtendedStatefulSet '%s' in namespace '%s'", statefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
	}

	r.log.Info("Created StatefulSet '", statefulSet.Name, "' for ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")

	return nil
}

// cleanupStatefulSets cleans up StatefulSets and versions if they are no longer required
func (r *ReconcileExtendedStatefulSet) cleanupStatefulSets(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet, maxAvailableVersion int, versions *map[int]bool) error {
	r.log.Info("Cleaning up StatefulSets for ExtendedStatefulSet '%s' less than version %d.", exStatefulSet.Name, maxAvailableVersion)

	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "Couldn't list StatefulSets for cleanup")
	}

	for _, statefulSet := range statefulSets {
		r.log.Debug("Considering StatefulSet '", statefulSet.Name, "' for cleanup.")

		strVersion, found := statefulSet.Annotations[essv1a1.AnnotationVersion]
		if !found {
			return errors.Errorf("Version annotation is not found from: %+v", statefulSet.Annotations)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return errors.Wrapf(err, "Version annotation is not an int: %s", strVersion)
		}

		if version >= maxAvailableVersion {
			continue
		}

		// Remove configs ownerReferences before delete
		existingConfigs, err := r.listConfigsOwnedBy(&statefulSet)
		if err != nil {
			return errors.Wrapf(err, "Could not list ConfigMaps and Secrets owned by %s", statefulSet.Name)
		}

		err = r.removeOwnerReferences(&statefulSet, existingConfigs)
		if err != nil {
			return errors.Wrapf(err, "Could not remove owner references of %s", statefulSet.Name)
		}

		r.log.Debug("Remove Finalizer from %s", statefulSet.Name)
		removeFinalizer(&statefulSet)

		err = r.client.Delete(context.TODO(), &statefulSet, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			r.log.Error("Could not delete StatefulSet  '", statefulSet.Name, "': ", err)
			return err
		}

		delete(*versions, version)
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

		result[version] = ready
	}

	return result, nil
}

// isStatefulSetReady returns true if one owned Pod is running
func (r *ReconcileExtendedStatefulSet) isStatefulSetReady(ctx context.Context, statefulSet *v1beta1.StatefulSet) (bool, error) {
	labelsSelector := labels.Set{
		"controller-revision-hash": statefulSet.Status.CurrentRevision,
	}

	podList := &corev1.PodList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     statefulSet.Namespace,
			LabelSelector: labelsSelector.AsSelector(),
		},
		podList,
	)
	if err != nil {
		return false, err
	}

	for _, pod := range podList.Items {
		if metav1.IsControlledBy(&pod, statefulSet) {
			if podUtils.IsPodReady(&pod) {
				r.log.Debug("Pod '", statefulSet.Name, "' owned by StatefulSet '", statefulSet.Name, "' is running.")
				return true, nil
			}
		}
	}

	return false, nil
}

// updateStatefulSetsConfigSHA1 Update StatefulSets configSHA1 and config OwnerReferences if necessary
func (r *ReconcileExtendedStatefulSet) updateStatefulSetsConfigSHA1(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet) error {
	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "List StatefulSets owned by %s/%s", exStatefulSet.GetNamespace(), exStatefulSet.GetName())
	}

	for _, statefulSet := range statefulSets {
		currentConfigRef, err := r.listConfigsFromSpec(&statefulSet)
		if err != nil {
			return errors.Wrapf(err, "Could not list ConfigMaps and Secrets from '%s' spec", statefulSet.Name)
		}

		existingConfigs, err := r.listConfigsOwnedBy(&statefulSet)
		if err != nil {
			return errors.Wrapf(err, "Could not list ConfigMaps and Secrets owned by '%s'", statefulSet.Name)
		}

		currentsha, err := calculateConfigHash(currentConfigRef)
		if err != nil {
			return err
		}

		err = r.updateOwnerReferences(&statefulSet, existingConfigs, currentConfigRef)
		if err != nil {
			return fmt.Errorf("error updating OwnerReferences: %v", err)
		}

		oldsha, _ := statefulSet.Annotations[essv1a1.AnnotationConfigSHA1]

		// If the current config sha doesn't match the existing config sha, update it
		if currentsha != oldsha {
			r.log.Debug("StatefulSet '", statefulSet.Name, "' configuration has changed.")

			err = r.updateConfigSHA1(&statefulSet, currentsha)
			if err != nil {
				return errors.Wrapf(err, "Update StatefulSet config sha1")
			}
			// TODO restart all pods ot the StatefulSet
		}
	}

	return nil

}

// listConfigsFromSpec returns a list of all Secrets and ConfigMaps that are
// referenced in the StatefulSet's spec
func (r *ReconcileExtendedStatefulSet) listConfigsFromSpec(statefulSet *v1beta1.StatefulSet) ([]essv1a1.Object, error) {
	r.log.Debug("Getting all ConfigMaps and Secrets that are referenced in '", statefulSet.Name, "' Spec.")
	configMaps, secrets := getConfigNamesFromSpec(statefulSet)

	// return error if config resource is not exist
	var configs []essv1a1.Object
	for name := range configMaps {
		key := types.NamespacedName{Namespace: statefulSet.GetNamespace(), Name: name}
		configMap := &corev1.ConfigMap{}
		err := r.client.Get(context.TODO(), key, configMap)
		if err != nil {
			return []essv1a1.Object{}, err
		}
		if configMap != nil {
			configs = append(configs, configMap)
		}
	}

	for name := range secrets {
		key := types.NamespacedName{Namespace: statefulSet.GetNamespace(), Name: name}
		secret := &corev1.Secret{}
		err := r.client.Get(context.TODO(), key, secret)
		if err != nil {
			return []essv1a1.Object{}, err
		}
		if secret != nil {
			configs = append(configs, secret)
		}
	}

	r.log.Debug(configs)
	return configs, nil
}

// getConfigNamesFromSpec parses the StatefulSet object and returns two sets,
// the first containing the names of all referenced ConfigMaps,
// the second containing the names of all referenced Secrets
func getConfigNamesFromSpec(statefulSet *v1beta1.StatefulSet) (map[string]struct{}, map[string]struct{}) {
	// Create sets for storing the names fo the ConfigMaps/Secrets
	configMaps := make(map[string]struct{})
	secrets := make(map[string]struct{})

	// Iterate over all Volumes and check the VolumeSources for ConfigMaps
	// and Secrets
	for _, vol := range statefulSet.Spec.Template.Spec.Volumes {
		if cm := vol.VolumeSource.ConfigMap; cm != nil {
			configMaps[cm.Name] = struct{}{}
		}
		if s := vol.VolumeSource.Secret; s != nil {
			secrets[s.SecretName] = struct{}{}
		}
	}

	// Iterate over all Containers and their respective EnvFrom and Env
	// then check the EnvFromSources for ConfigMaps and Secrets
	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		for _, env := range container.EnvFrom {
			if cm := env.ConfigMapRef; cm != nil {
				configMaps[cm.Name] = struct{}{}
			}
			if s := env.SecretRef; s != nil {
				secrets[s.Name] = struct{}{}
			}
		}

		for _, env := range container.Env {
			if cmRef := env.ValueFrom.ConfigMapKeyRef; cmRef != nil {
				configMaps[cmRef.Name] = struct{}{}

			}
			if sRef := env.ValueFrom.SecretKeyRef; sRef != nil {
				secrets[sRef.Name] = struct{}{}

			}
		}
	}

	return configMaps, secrets
}

// listConfigsOwnedBy returns a list of all ConfigMaps and Secrets that are
// owned by the StatefulSet instance
func (r *ReconcileExtendedStatefulSet) listConfigsOwnedBy(statefulSet *v1beta1.StatefulSet) ([]essv1a1.Object, error) {
	r.log.Debug("Getting all ConfigMaps and Secrets that are owned by '", statefulSet.Name, "'.")
	opts := client.InNamespace(statefulSet.GetNamespace())

	// List all ConfigMaps in the StatefulSet's namespace
	configMaps := &corev1.ConfigMapList{}
	err := r.client.List(context.TODO(), opts, configMaps)
	if err != nil {
		return []essv1a1.Object{}, fmt.Errorf("error listing ConfigMaps: %v", err)
	}

	// List all Secrets in the StatefulSet's namespace
	secrets := &corev1.SecretList{}
	err = r.client.List(context.TODO(), opts, secrets)
	if err != nil {
		return []essv1a1.Object{}, fmt.Errorf("error listing Secrets: %v", err)
	}

	// Iterate over the ConfigMaps/Secrets and add the ones owned by the
	// StatefulSet to the output list configs
	configs := []essv1a1.Object{}
	for _, cm := range configMaps.Items {
		if isOwnedBy(&cm, statefulSet) {
			configs = append(configs, cm.DeepCopy())
		}
	}
	for _, s := range secrets.Items {
		if isOwnedBy(&s, statefulSet) {
			configs = append(configs, s.DeepCopy())
		}
	}

	return configs, nil
}

// isOwnedBy returns true if the child has an owner reference that points to
// the owner object
func isOwnedBy(child, owner essv1a1.Object) bool {
	for _, ref := range child.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

// calculateConfigHash calculates the SHA1 of the JSON representation of configuration objects
func calculateConfigHash(children []essv1a1.Object) (string, error) {
	// hashSource contains all the data to be hashed
	hashSource := struct {
		ConfigMaps map[string]map[string]string `json:"configMaps"`
		Secrets    map[string]map[string][]byte `json:"secrets"`
	}{
		ConfigMaps: make(map[string]map[string]string),
		Secrets:    make(map[string]map[string][]byte),
	}

	// Add the data from each child to the hashSource
	// All children should be in the same namespace so each one should have a
	// unique name
	for _, obj := range children {
		switch child := obj.(type) {
		case *corev1.ConfigMap:
			cm := corev1.ConfigMap(*child)
			hashSource.ConfigMaps[cm.GetName()] = cm.Data
		case *corev1.Secret:
			s := corev1.Secret(*child)
			hashSource.Secrets[s.GetName()] = s.Data
		default:
			return "", fmt.Errorf("passed unknown type: %v", reflect.TypeOf(child))
		}
	}

	// Convert the hashSource to a byte slice so that it can be hashed
	hashSourceBytes, err := json.Marshal(hashSource)
	if err != nil {
		return "", fmt.Errorf("unable to marshal JSON: %v", err)
	}

	return fmt.Sprintf("%x", sha1.Sum(hashSourceBytes)), nil
}

// updateConfigSHA1 updates the configuration sha1 of the given StatefulSet to the
// given string
func (r *ReconcileExtendedStatefulSet) updateConfigSHA1(actualStatefulSet *v1beta1.StatefulSet, hash string) error {
	key := types.NamespacedName{Namespace: actualStatefulSet.GetNamespace(), Name: actualStatefulSet.GetName()}
	err := r.client.Get(context.TODO(), key, actualStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "Could not get StatefulSet '", actualStatefulSet.GetName(), "'")
	}
	// Get the existing annotations
	annotations := actualStatefulSet.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Update the annotations
	annotations[essv1a1.AnnotationConfigSHA1] = hash
	actualStatefulSet.SetAnnotations(annotations)

	r.log.Debug("Updating new config sha1 for StatefulSet '", actualStatefulSet.GetName(), "'.")
	err = r.client.Update(context.TODO(), actualStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "Could not update StatefulSet '", actualStatefulSet.GetName(), "'")
	}

	return nil
}

// updateOwnerReferences determines which children need to have their
// OwnerReferences added/updated and which need to have their OwnerReferences
// removed and then performs all updates
func (r *ReconcileExtendedStatefulSet) updateOwnerReferences(owner *v1beta1.StatefulSet, existing, current []essv1a1.Object) error {
	r.log.Debug("Updating ownerReferences for StatefulSet '", owner.Name, "' in namespace '", owner.Namespace, "'.")

	// Add an owner reference to each child object
	for _, obj := range current {
		err := r.updateOwnerReference(owner, obj)
		if err != nil {
			return errors.Wrapf(err, "Could not update Owner References")
		}
	}

	// Get the orphaned children and remove their OwnerReferences
	orphans := getOrphans(existing, current)
	err := r.removeOwnerReferences(owner, orphans)
	if err != nil {
		return errors.Wrapf(err, "Could not remove Owner References")
	}

	return nil
}

// removeOwnerReferences iterates over a list of children and removes the owner
// reference from the child before updating it
func (r *ReconcileExtendedStatefulSet) removeOwnerReferences(obj *v1beta1.StatefulSet, children []essv1a1.Object) error {
	for _, child := range children {
		// Filter the existing ownerReferences
		ownerRefs := []metav1.OwnerReference{}
		for _, ref := range child.GetOwnerReferences() {
			if ref.UID != obj.UID {
				ownerRefs = append(ownerRefs, ref)
			}
		}

		// Compare the ownerRefs and update if they have changed
		if !reflect.DeepEqual(ownerRefs, child.GetOwnerReferences()) {
			child.SetOwnerReferences(ownerRefs)
			r.log.Debug("Removing child '", child.GetName(), "' from StatefulSet '", obj.Name, "' in namespace '", obj.Namespace, "'.")
			err := r.client.Update(context.TODO(), child)
			if err != nil {
				r.log.Error("Could not update '", child.GetName(), "': ", err)
				return err
			}
		}
	}
	return nil
}

// updateOwnerReference ensures that the child object has an OwnerReference
// pointing to the owner
func (r *ReconcileExtendedStatefulSet) updateOwnerReference(owner *v1beta1.StatefulSet, child essv1a1.Object) error {
	ownerRef := getOwnerReference(owner)
	for _, ref := range child.GetOwnerReferences() {
		// Owner Reference already exists, do nothing
		if reflect.DeepEqual(ref, ownerRef) {
			return nil
		}
	}

	// Append the new OwnerReference and update the child
	ownerRefs := append(child.GetOwnerReferences(), ownerRef)
	child.SetOwnerReferences(ownerRefs)

	r.log.Debug("Updating child '", child.GetObjectKind().GroupVersionKind().Kind, "/", child.GetName(), "' for StatefulSet '", owner.Name, "' in namespace '", owner.Namespace, "'.")
	err := r.client.Update(context.TODO(), child)
	if err != nil {
		r.log.Error("Could not update '", child.GetObjectKind().GroupVersionKind().Kind, "/", child.GetName(), "': ", err)
		return err
	}
	return nil
}

// handleDelete removes all existing Owner References pointing to ExtendedStatefulSet's StatefulSets
// and object's Finalizers
func (r *ReconcileExtendedStatefulSet) handleDelete(extendedStatefulSet *essv1a1.ExtendedStatefulSet) (reconcile.Result, error) {
	statefulSets, err := r.listStatefulSets(context.TODO(), extendedStatefulSet)
	if err != nil {
		r.log.Error("Could not retrieve all StatefulSets owned by ExtendedStatefulSet '", extendedStatefulSet.Name, "': ", err)
		return reconcile.Result{}, err
	}

	// Handle child deletion
	for _, statefulSet := range statefulSets {
		r.log.Debug("Considering existing Owner References of StatefulSet '", statefulSet.Name, "'.")

		// Fetch all ConfigMaps and Secrets with an OwnerReference pointing to the object
		existingConfigs, err := r.listConfigsOwnedBy(&statefulSet)
		if err != nil {
			r.log.Error("Could not retrieve all ConfigMaps and Secrets owned by StatefulSet '", statefulSet.Name, "': ", err)
			return reconcile.Result{}, err
		}

		// Remove StatefulSet OwnerReferences from the existingConfigs
		err = r.removeOwnerReferences(&statefulSet, existingConfigs)
		if err != nil {
			r.log.Error("Could not remove OwnerReferences pointing to StatefulSet '", statefulSet.Name, "': ", err)
			return reconcile.Result{}, err
		}

		// Remove the object's Finalizer and update if necessary
		copy := statefulSet.DeepCopy()
		removeFinalizer(copy)
		if !reflect.DeepEqual(statefulSet, copy) {
			r.log.Debug("Removing finalizer from StatefulSet '", statefulSet.Name, "'.")
			err := r.client.Update(context.TODO(), copy)
			if err != nil {
				r.log.Error("Could not remove Finalizer from StatefulSet '", extendedStatefulSet.Name, "': ", err)
				return reconcile.Result{}, err
			}
		}
	}

	// Remove the object's Finalizer and update if necessary
	copy := extendedStatefulSet.DeepCopy()
	copy.RemoveFinalizer()
	if !reflect.DeepEqual(extendedStatefulSet, copy) {
		r.log.Debug("Removing Finalizer from ExtendedStatefulSet '", copy.Name, "'.")
		key := types.NamespacedName{Namespace: copy.GetNamespace(), Name: copy.GetName()}
		err := r.client.Get(context.TODO(), key, copy)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "Could not get StatefulSet '", copy.GetName(), "'")
		}

		copy.RemoveFinalizer()

		err = r.client.Update(context.TODO(), copy)
		if err != nil {
			r.log.Error("Could not remove Finalizer from ExtendedStatefulSet '", copy.GetName(), "': ", err)
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// getOwnerReference constructs an OwnerReference pointing to the StatefulSet
func getOwnerReference(statefulSet *v1beta1.StatefulSet) metav1.OwnerReference {
	t := true
	f := false
	return metav1.OwnerReference{
		APIVersion:         "apps/v1",
		Kind:               "StatefulSet",
		Name:               statefulSet.GetName(),
		UID:                statefulSet.GetUID(),
		BlockOwnerDeletion: &t,
		Controller:         &f,
	}
}

// getOrphans creates a slice of orphaned objects that need their
// OwnerReferences removing
func getOrphans(existing, current []essv1a1.Object) []essv1a1.Object {
	orphans := []essv1a1.Object{}
	for _, child := range existing {
		if !isIn(current, child) {
			orphans = append(orphans, child)
		}
	}
	return orphans
}

// isIn checks whether a child object exists within a slice of objects
func isIn(list []essv1a1.Object, child essv1a1.Object) bool {
	for _, obj := range list {
		if obj.GetUID() == child.GetUID() {
			return true
		}
	}
	return false
}

// addFinalizer adds the finalizer to the given StatefulSet
func addFinalizer(statefulSet *v1beta1.StatefulSet) {
	finalizers := statefulSet.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == essv1a1.FinalizerString {
			return
		}
	}

	finalizers = append(finalizers, essv1a1.FinalizerString)
	statefulSet.SetFinalizers(finalizers)
}

// removeFinalizer removes the finalizer from the given StatefulSet
func removeFinalizer(statefulSet *v1beta1.StatefulSet) {
	finalizers := statefulSet.GetFinalizers()

	newFinalizers := []string{}
	for _, finalizer := range finalizers {
		if finalizer != essv1a1.FinalizerString {
			newFinalizers = append(newFinalizers, finalizer)
		}
	}

	statefulSet.SetFinalizers(newFinalizers)
}
