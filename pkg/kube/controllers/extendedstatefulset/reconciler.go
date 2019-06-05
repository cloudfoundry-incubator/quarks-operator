package extendedstatefulset

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

const (
	// OptimisticLockErrorMsg is an error message shown when locking fails
	OptimisticLockErrorMsg = "the object has been modified; please apply your changes to the latest version and try again"
	// EnvKubeAz is set by available zone name
	EnvKubeAz = "KUBE_AZ"
	// EnvBoshAz is set by available zone name
	EnvBoshAz = "BOSH_AZ"
	// EnvReplicas describes the number of replicas in the ExtendedStatefulSet
	EnvReplicas = "REPLICAS"
	// EnvCfOperatorAz is set by available zone name
	EnvCfOperatorAz = "CF_OPERATOR_AZ"
	// EnvCfOperatorAzIndex is set by available zone index
	EnvCfOperatorAzIndex = "AZ_INDEX"
)

// Check that ReconcileExtendedStatefulSet implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileExtendedStatefulSet{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// Owner bundles funcs to manage ownership on referenced configmaps and secrets
type Owner interface {
	Update(context.Context, apis.Object, []apis.Object, []apis.Object) error
	RemoveOwnerReferences(context.Context, apis.Object, []apis.Object) error
	ListConfigs(context.Context, string, corev1.PodSpec) ([]apis.Object, error)
	ListConfigsOwnedBy(context.Context, apis.Object) ([]apis.Object, error)
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, srf setReferenceFunc) reconcile.Reconciler {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileExtendedStatefulSet{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		setReference:         srf,
		owner:                owner.NewOwner(mgr.GetClient(), mgr.GetScheme()),
		versionedSecretStore: versionedSecretStore,
	}
}

// ReconcileExtendedStatefulSet reconciles an ExtendedStatefulSet object
type ReconcileExtendedStatefulSet struct {
	ctx                  context.Context
	client               client.Client
	scheme               *runtime.Scheme
	setReference         setReferenceFunc
	config               *config.Config
	owner                Owner
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// Reconcile reads that state of the cluster for a ExtendedStatefulSet object
// and makes changes based on the state read and what is in the ExtendedStatefulSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileExtendedStatefulSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the ExtendedStatefulSet we need to reconcile
	exStatefulSet := &estsv1.ExtendedStatefulSet{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling ExtendedStatefulSet ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, exStatefulSet)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Debug(ctx, "Skip reconcile: ExtendedStatefulSet not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Cleanup volumemanagement statefulset once it's all pods are ready
	err = r.deleteVolumeManagementStatefulSet(ctx, exStatefulSet)
	if err != nil {
		ctxlog.Error(ctx, "Could not delete volumemanagement statefulset of ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	// Get the actual StatefulSet
	actualStatefulSet, actualVersion, err := r.getActualStatefulSet(ctx, exStatefulSet)
	if err != nil {
		return reconcile.Result{}, ctxlog.WithEvent(exStatefulSet, "StatefulSetNotFound").Error(ctx, "Could not retrieve latest StatefulSet owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
	}

	// Calculate the desired statefulSets
	desiredStatefulSets, desiredVersion, err := r.calculateDesiredStatefulSets(exStatefulSet, actualStatefulSet)
	if err != nil {
		return reconcile.Result{}, ctxlog.WithEvent(exStatefulSet, "CalculationError").Error(ctx, "Could not calculate StatefulSet owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
	}

	if exStatefulSet.Spec.Template.Spec.VolumeClaimTemplates != nil {
		err := r.alterVolumeManagementStatefulSet(ctx, actualVersion, desiredVersion, exStatefulSet, actualStatefulSet)
		if err != nil {
			ctxlog.Error(ctx, "Alteration of VolumeManagement statefulset failed for ExtendedStatefulset ", exStatefulSet.Name, " in namespace ", exStatefulSet.Namespace, ".", err)
			return reconcile.Result{}, err
		}
	}

	for _, desiredStatefulSet := range desiredStatefulSets {

		desiredStatefulSet.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{}

		// If actual version is zero, there is no StatefulSet live
		if actualVersion != desiredVersion {

			// If it doesn't exist, create it
			ctxlog.Info(ctx, "StatefulSet '", desiredStatefulSet.Name, "' owned by ExtendedStatefulSet '", request.NamespacedName, "' not found, will be created.")

			// Record the template before creating the StatefulSet, so we don't include default values such as
			// `ImagePullPolicy`, `TerminationMessagePath`, etc. in the signature.
			originalTemplate := exStatefulSet.Spec.Template.DeepCopy()
			if err := r.createStatefulSet(ctx, exStatefulSet, &desiredStatefulSet); err != nil {
				return reconcile.Result{}, ctxlog.WithEvent(exStatefulSet, "CreateStatefulSetError").Error(ctx, "Could not create StatefulSet for ExtendedStatefulSet '", request.NamespacedName, "': ", err)
			}
			exStatefulSet.Spec.Template = *originalTemplate
		} else {
			// If it does exist, do a deep equal and check that we own it
			ctxlog.Info(ctx, "StatefulSet '", desiredStatefulSet.Name, "' owned by ExtendedStatefulSet '", request.NamespacedName, "' has not changed, checking if any other changes are necessary.")
		}
	}

	statefulSetVersions, err := r.listStatefulSetVersions(ctx, exStatefulSet)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Update StatefulSets configSHA1 and trigger statefulSet rollingUpdate if necessary
	if exStatefulSet.Spec.UpdateOnConfigChange {
		ctxlog.Debugf(ctx, "Considering configurations to trigger update.")

		err = r.updateStatefulSetsConfigSHA1(ctx, exStatefulSet)
		if err != nil {
			return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, ctxlog.WithEvent(exStatefulSet, "UpdateError").Error(ctx, "Could not update StatefulSets owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		}
	}

	// Update the status of the resource
	if !reflect.DeepEqual(statefulSetVersions, exStatefulSet.Status.Versions) {
		ctxlog.Debugf(ctx, "Updating ExtendedStatefulSet '%s'", request.NamespacedName)
		exStatefulSet.Status.Versions = statefulSetVersions
		updateErr := r.client.Update(ctx, exStatefulSet)
		if updateErr != nil {
			ctxlog.WithEvent(exStatefulSet, "UpdateError").Errorf(ctx, "Failed to update exStatefulSet status: %v", updateErr)
		}
	}

	maxAvailableVersion := exStatefulSet.GetMaxAvailableVersion(statefulSetVersions)

	if len(statefulSetVersions) > 1 {
		// Cleanup versions smaller than the max available version
		err = r.cleanupStatefulSets(ctx, exStatefulSet, maxAvailableVersion, &statefulSetVersions)
		if err != nil {
			ctxlog.WithEvent(exStatefulSet, "CleanupError").Error(ctx, "Could not cleanup StatefulSets owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
			return reconcile.Result{}, err
		}
	}

	if !statefulSetVersions[desiredVersion] {
		ctxlog.Debug(ctx, "Waiting for the desired version to become available for ExtendedStatefulSet ", request.NamespacedName)
		return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	// Reconcile stops since only one version or no version exists.
	ctxlog.Debug(ctx, "Version '", desiredVersion, "' is available")
	return reconcile.Result{}, nil
}

// calculateDesiredStatefulSets generates the desired StatefulSets that should exist
func (r *ReconcileExtendedStatefulSet) calculateDesiredStatefulSets(exStatefulSet *estsv1.ExtendedStatefulSet, actualStatefulSet *v1beta2.StatefulSet) ([]v1beta2.StatefulSet, int, error) {
	var desiredStatefulSets []v1beta2.StatefulSet

	template := exStatefulSet.Spec.Template.DeepCopy()

	// Place the StatefulSet in the same namespace as the ExtendedStatefulSet
	template.SetNamespace(exStatefulSet.Namespace)

	desiredVersion, err := exStatefulSet.DesiredVersion(actualStatefulSet)
	if err != nil {
		return nil, 0, err
	}

	templateSHA1, err := exStatefulSet.CalculateStatefulSetSHA1()
	if err != nil {
		return nil, 0, err
	}

	// Set version and template SHA1
	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}

	template.Annotations[estsv1.AnnotationStatefulSetSHA1] = templateSHA1
	template.Annotations[estsv1.AnnotationVersion] = fmt.Sprintf("%d", desiredVersion)

	if exStatefulSet.Spec.ZoneNodeLabel == "" {
		exStatefulSet.Spec.ZoneNodeLabel = estsv1.DefaultZoneNodeLabel
	}

	if len(exStatefulSet.Spec.Zones) > 0 {
		for zoneIndex, zoneName := range exStatefulSet.Spec.Zones {
			statefulSet, err := r.generateSingleStatefulSet(exStatefulSet, template, zoneIndex, zoneName, desiredVersion, templateSHA1)
			if err != nil {
				return desiredStatefulSets, desiredVersion, errors.Wrapf(err, "Could not generate StatefulSet template for AZ '%d/%s'", zoneIndex, zoneName)
			}
			desiredStatefulSets = append(desiredStatefulSets, *statefulSet)
		}

	} else {
		statefulSet, err := r.generateSingleStatefulSet(exStatefulSet, template, 0, "", desiredVersion, templateSHA1)
		if err != nil {
			return desiredStatefulSets, desiredVersion, errors.Wrap(err, "Could not generate StatefulSet template for single zone")
		}
		desiredStatefulSets = append(desiredStatefulSets, *statefulSet)
	}

	return desiredStatefulSets, desiredVersion, nil
}

// createStatefulSet creates a StatefulSet
func (r *ReconcileExtendedStatefulSet) createStatefulSet(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet, statefulSet *v1beta2.StatefulSet) error {

	// Set the owner of the StatefulSet, so it's garbage collected,
	// and we can find it later
	ctxlog.Info(ctx, "Setting owner for StatefulSet '", statefulSet.Name, "' to ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")
	if err := r.setReference(exStatefulSet, statefulSet, r.scheme); err != nil {
		return errors.Wrapf(err, "could not set owner for StatefulSet '%s' to ExtendedStatefulSet '%s' in namespace '%s'", statefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
	}

	// Create the StatefulSet
	if err := r.client.Create(ctx, statefulSet); err != nil {
		return errors.Wrapf(err, "could not create StatefulSet '%s' for ExtendedStatefulSet '%s' in namespace '%s'", statefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
	}

	ctxlog.Info(ctx, "Created StatefulSet '", statefulSet.Name, "' for ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")

	return nil
}

// cleanupStatefulSets cleans up StatefulSets and versions if they are no longer required
func (r *ReconcileExtendedStatefulSet) cleanupStatefulSets(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet, maxAvailableVersion int, versions *map[int]bool) error {
	ctxlog.WithEvent(exStatefulSet, "CleanupStatefulSets").Infof(ctx, "Cleaning up StatefulSets for ExtendedStatefulSet '%s' less than version %d.", exStatefulSet.Name, maxAvailableVersion)

	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "couldn't list StatefulSets for cleanup")
	}

	for _, statefulSet := range statefulSets {
		ctxlog.Debug(ctx, "Considering StatefulSet '", statefulSet.Name, "' for cleanup.")

		strVersion, found := statefulSet.Annotations[estsv1.AnnotationVersion]
		if !found {
			return errors.Errorf("version annotation is not found from: %+v", statefulSet.Annotations)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return errors.Wrapf(err, "version annotation is not an int: %s", strVersion)
		}

		if version >= maxAvailableVersion {
			continue
		}

		ctxlog.Debugf(ctx, "Deleting StatefulSet '%s'", statefulSet.Name)
		err = r.client.Delete(ctx, &statefulSet, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			ctxlog.WithEvent(exStatefulSet, "DeleteError").Error(ctx, "Could not delete StatefulSet  '", statefulSet.Name, "': ", err)
			return err
		}

		delete(*versions, version)
	}

	return nil
}

// listStatefulSets gets all StatefulSets owned by the ExtendedStatefulSet
func (r *ReconcileExtendedStatefulSet) listStatefulSets(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	result := []v1beta2.StatefulSet{}

	// Get owned resources
	// Go through each StatefulSet
	allStatefulSets := &v1beta2.StatefulSetList{}
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
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")
		} else {
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' is not owned by ExtendedStatefulSet '", exStatefulSet.Name, "', ignoring.")
		}
	}

	return result, nil
}

// getActualStatefulSet gets the latest (by version) StatefulSet owned by the ExtendedStatefulSet
func (r *ReconcileExtendedStatefulSet) getActualStatefulSet(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet) (*v1beta2.StatefulSet, int, error) {
	// Default response is an empty StatefulSet with version '0' and an empty signature
	result := &v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				estsv1.AnnotationStatefulSetSHA1: "",
				estsv1.AnnotationVersion:         "0",
			},
		},
	}
	maxVersion := 0

	// Get all owned StatefulSets
	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return nil, 0, err
	}

	ctxlog.Debug(ctx, "Getting the latest StatefulSet owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	for _, ss := range statefulSets {
		strVersion := ss.Annotations[estsv1.AnnotationVersion]
		if strVersion == "" {
			return nil, 0, errors.New(fmt.Sprintf("The statefulset '%s' does not have the annotation('%s'), a version could not be retrieved.", ss.Name, estsv1.AnnotationVersion))
		}

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
func (r *ReconcileExtendedStatefulSet) listStatefulSetVersions(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet) (map[int]bool, error) {
	result := map[int]bool{}

	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return nil, err
	}

	for _, statefulSet := range statefulSets {
		strVersion, found := statefulSet.Annotations[estsv1.AnnotationVersion]
		if !found {
			return result, errors.Errorf("version annotation is not found from: %+v", statefulSet.Annotations)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return result, errors.Wrapf(err, "version annotation is not an int: %s", strVersion)
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
func (r *ReconcileExtendedStatefulSet) isStatefulSetReady(ctx context.Context, statefulSet *v1beta2.StatefulSet) (bool, error) {
	labelsSelector := labels.Set{
		v1beta2.StatefulSetRevisionLabel: statefulSet.Status.CurrentRevision,
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
			if podutil.IsPodReady(&pod) {
				ctxlog.Debug(ctx, "Pod '", statefulSet.Name, "' owned by StatefulSet '", statefulSet.Name, "' is running.")
				return true, nil
			}
		}
	}

	return false, nil
}

// updateStatefulSetsConfigSHA1 Update StatefulSets configSHA1 and config OwnerReferences if necessary
func (r *ReconcileExtendedStatefulSet) updateStatefulSetsConfigSHA1(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet) error {
	statefulSets, err := r.listStatefulSets(ctx, exStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "list StatefulSets owned by %s/%s", exStatefulSet.GetNamespace(), exStatefulSet.GetName())
	}

	for _, statefulSet := range statefulSets {
		ctxlog.Debug(ctx, "Getting all ConfigMaps and Secrets that are referenced in '", statefulSet.Name, "' Spec.")

		namespace := statefulSet.GetNamespace()

		currentConfigRef, err := r.owner.ListConfigs(ctx, namespace, statefulSet.Spec.Template.Spec)
		if err != nil {
			return errors.Wrapf(err, "could not list ConfigMaps and Secrets from '%s' spec", statefulSet.Name)
		}

		currentsha, err := calculateConfigHash(currentConfigRef)
		if err != nil {
			return err
		}

		oldsha := statefulSet.Spec.Template.Annotations[estsv1.AnnotationConfigSHA1]

		// If the current config sha doesn't match the existing config sha, update it
		if currentsha != oldsha {
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' configuration has changed.")

			err = r.updateConfigSHA1(ctx, &statefulSet, currentsha)
			if err != nil {
				return errors.Wrapf(err, "update StatefulSet config sha1")
			}
		}
	}

	return nil
}

// calculateConfigHash calculates the SHA1 of the JSON representation of configuration objects
func calculateConfigHash(children []apis.Object) (string, error) {
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
func (r *ReconcileExtendedStatefulSet) updateConfigSHA1(ctx context.Context, actualStatefulSet *v1beta2.StatefulSet, hash string) error {
	// Get the existing annotations
	annotations := actualStatefulSet.Spec.Template.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Update the annotations
	annotations[estsv1.AnnotationConfigSHA1] = hash

	ctxlog.Debug(ctx, "Updating new config sha1 for StatefulSet '", actualStatefulSet.Name, "'.")
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, actualStatefulSet.DeepCopy(), func(obj runtime.Object) error {
		if existingSts, ok := obj.(*v1beta2.StatefulSet); ok {
			actualStatefulSet.ObjectMeta.ResourceVersion = existingSts.ObjectMeta.ResourceVersion
			actualStatefulSet.Spec.Template.SetAnnotations(annotations)
			actualStatefulSet.DeepCopyInto(existingSts)
			return nil
		}
		return fmt.Errorf("object is not an ExtendStatefulSet")
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to apply StatefulSet '%s': %v", actualStatefulSet.Name, err)
	}

	return nil
}

// generateSingleStatefulSet creates a StatefulSet from one zone
func (r *ReconcileExtendedStatefulSet) generateSingleStatefulSet(exStatefulSet *estsv1.ExtendedStatefulSet, template *v1beta2.StatefulSet, zoneIndex int, zoneName string, version int, templateSha1 string) (*v1beta2.StatefulSet, error) {
	statefulSet := template.DeepCopy()

	// Get the labels and annotations
	labels := statefulSet.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	annotations := statefulSet.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	statefulSetNamePrefix := exStatefulSet.GetName()

	// Get the pod labels and annotations
	podLabels := statefulSet.Spec.Template.GetLabels()
	if podLabels == nil {
		podLabels = make(map[string]string)
	}
	podAnnotations := statefulSet.Spec.Template.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}

	// Update available-zone specified properties
	if zoneName != "" {
		// Reset name prefix with zoneIndex
		statefulSetNamePrefix = fmt.Sprintf("%s-z%d", exStatefulSet.GetName(), zoneIndex)

		labels[estsv1.LabelAZIndex] = strconv.Itoa(zoneIndex)
		labels[estsv1.LabelAZName] = zoneName

		zonesBytes, err := json.Marshal(exStatefulSet.Spec.Zones)
		if err != nil {
			return &v1beta2.StatefulSet{}, errors.Wrapf(err, "Could not marshal zones: '%v'", exStatefulSet.Spec.Zones)
		}
		annotations[estsv1.AnnotationZones] = string(zonesBytes)

		podLabels[estsv1.LabelAZIndex] = strconv.Itoa(zoneIndex)
		podLabels[estsv1.LabelAZName] = zoneName

		podAnnotations[estsv1.AnnotationZones] = string(zonesBytes)

		statefulSet = r.updateAffinity(statefulSet, exStatefulSet.Spec.ZoneNodeLabel, zoneIndex, zoneName)
	}

	// Set az-index as 0 for single zoneName
	podLabels[estsv1.LabelAZIndex] = strconv.Itoa(zoneIndex)

	statefulSet.Spec.Template.SetLabels(podLabels)
	statefulSet.Spec.Template.SetAnnotations(podAnnotations)

	r.injectContainerEnv(&statefulSet.Spec.Template.Spec, zoneIndex, zoneName, exStatefulSet.Spec.Template.Spec.Replicas)

	annotations[estsv1.AnnotationStatefulSetSHA1] = templateSha1
	annotations[estsv1.AnnotationVersion] = fmt.Sprintf("%d", version)

	// Set updated properties
	statefulSet.SetName(fmt.Sprintf("%s-v%d", statefulSetNamePrefix, version))
	statefulSet.SetLabels(labels)
	statefulSet.SetAnnotations(annotations)

	return statefulSet, nil
}

// updateAffinity Update current statefulSet Affinity from AZ specification
func (r *ReconcileExtendedStatefulSet) updateAffinity(statefulSet *v1beta2.StatefulSet, zoneNodeLabel string, zoneIndex int, zoneName string) *v1beta2.StatefulSet {
	nodeInZoneSelector := corev1.NodeSelectorRequirement{
		Key:      zoneNodeLabel,
		Operator: corev1.NodeSelectorOpIn,
		Values:   []string{zoneName},
	}

	affinity := statefulSet.Spec.Template.Spec.Affinity
	// Check if optional properties were set
	if affinity == nil {
		affinity = &corev1.Affinity{}
	}

	if affinity.NodeAffinity == nil {
		affinity.NodeAffinity = &corev1.NodeAffinity{}
	}

	if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						nodeInZoneSelector,
					},
				},
			},
		}
	} else {
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				nodeInZoneSelector,
			},
		})
	}

	statefulSet.Spec.Template.Spec.Affinity = affinity

	return statefulSet
}

// injectContainerEnv inject AZ info to container envs
func (r *ReconcileExtendedStatefulSet) injectContainerEnv(podSpec *corev1.PodSpec, zoneIndex int, zoneName string, replicas *int32) {

	containers := []*corev1.Container{}
	for i := 0; i < len(podSpec.Containers); i++ {
		containers = append(containers, &podSpec.Containers[i])
	}
	for i := 0; i < len(podSpec.InitContainers); i++ {
		containers = append(containers, &podSpec.InitContainers[i])
	}
	for _, container := range containers {
		envs := container.Env

		if zoneIndex >= 0 {
			envs = upsertEnvs(envs, EnvKubeAz, zoneName)
			envs = upsertEnvs(envs, EnvBoshAz, zoneName)
			envs = upsertEnvs(envs, EnvCfOperatorAz, zoneName)
			envs = upsertEnvs(envs, EnvCfOperatorAzIndex, strconv.Itoa(zoneIndex+1))
		} else {
			// Default to zone 1
			envs = upsertEnvs(envs, EnvCfOperatorAzIndex, "1")
		}
		envs = upsertEnvs(envs, EnvReplicas, strconv.Itoa(int(*replicas)))

		container.Env = envs
	}
}

func upsertEnvs(envs []corev1.EnvVar, name string, value string) []corev1.EnvVar {
	for idx, env := range envs {
		if env.Name == name {
			envs[idx].Value = value
			return envs
		}
	}

	envs = append(envs, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	return envs
}
