package owner

import (
	"fmt"
	"reflect"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Owner helps managing ownership of configmaps and secrets, which are
// referenced in a pod spec
type Owner struct {
	client client.Client
	scheme *runtime.Scheme
	log    *zap.SugaredLogger
}

// NewOwner returns a newly initialized owner
func NewOwner(
	client client.Client,
	log *zap.SugaredLogger,
	scheme *runtime.Scheme,
) Owner {
	return Owner{
		client: client,
		log:    log,
		scheme: scheme,
	}
}

// Update determines which children need to have their OwnerReferences
// added/updated and which need to have their OwnerReferences removed and then
// performs all updates
func (r Owner) Update(ctx context.Context, owner apis.Object, existing, current []apis.Object) error {

	// Add an owner reference to each child object
	ownerRef, err := buildOwnerReference(r.scheme, owner)
	if err != nil {
		return errors.Wrapf(err, "could not get Owner Reference")
	}

	for _, obj := range current {
		err := r.updateOwnerReference(ctx, ownerRef, obj)
		if err != nil {
			return errors.Wrapf(err, "could not update Owner References")
		}
	}

	// Get the orphaned children and remove their OwnerReferences
	orphans := getOrphans(existing, current)
	err = r.RemoveOwnerReferences(ctx, owner, orphans)
	if err != nil {
		return errors.Wrapf(err, "could not remove Owner References")
	}

	return nil
}

// RemoveOwnerReferences iterates over a list of children and removes the
// owner reference from the child before updating it
func (r Owner) RemoveOwnerReferences(ctx context.Context, obj apis.Object, children []apis.Object) error {
	for _, child := range children {
		// Filter owner from the existing ownerReferences
		ownerRefs := []metav1.OwnerReference{}
		for _, ref := range child.GetOwnerReferences() {
			if ref.UID != obj.GetUID() {
				ownerRefs = append(ownerRefs, ref)
			}
		}

		// Compare the ownerRefs and update if they have changed
		if !reflect.DeepEqual(ownerRefs, child.GetOwnerReferences()) {
			child.SetOwnerReferences(ownerRefs)
			r.log.Debug("Removing child '", child.GetNamespace(), "/", child.GetName(), "' from StatefulSet '", obj.GetName(), "' in namespace '", obj.GetNamespace(), "'.")
			err := r.client.Update(ctx, child)
			if err != nil {
				r.log.Error("could not update '", child.GetName(), "': ", err)
				return err
			}
		}
	}
	return nil
}

// updateOwnerReference ensures that the child object has an OwnerReference
// pointing to the owner
func (r Owner) updateOwnerReference(ctx context.Context, ownerRef metav1.OwnerReference, child apis.Object) error {
	for _, ref := range child.GetOwnerReferences() {
		// Owner Reference already exists, do nothing
		if reflect.DeepEqual(ref, ownerRef) {
			return nil
		}
	}

	// Append the new OwnerReference and update the child
	ownerRefs := append(child.GetOwnerReferences(), ownerRef)
	child.SetOwnerReferences(ownerRefs)

	r.log.Debug("Updating child '", child.GetObjectKind().GroupVersionKind().Kind, "/", child.GetName(), "' for owner '", ownerRef.Name, "'.")
	err := r.client.Update(ctx, child)
	if err != nil {
		r.log.Error("could not update '", child.GetObjectKind().GroupVersionKind().Kind, "/", child.GetName(), "': ", err)
		return err
	}
	return nil
}

// buildOwnerReference constructs an OwnerReference pointing to the owner
func buildOwnerReference(scheme *runtime.Scheme, owner apis.Object) (metav1.OwnerReference, error) {
	ro, ok := owner.(runtime.Object)
	if !ok {
		return metav1.OwnerReference{}, fmt.Errorf("is not a %T a runtime.Object, cannot call SetControllerReference", owner)
	}

	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return metav1.OwnerReference{}, err
	}

	t := true
	f := false
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: &t,
		Controller:         &f,
	}, nil
}

// getOrphans creates a slice of orphaned objects that need their
// OwnerReferences removing
func getOrphans(existing, current []apis.Object) []apis.Object {
	orphans := []apis.Object{}
	for _, child := range existing {
		if !isIn(current, child) {
			orphans = append(orphans, child)
		}
	}
	return orphans
}

// isIn checks whether a child object exists within a slice of objects
func isIn(list []apis.Object, child apis.Object) bool {
	for _, obj := range list {
		if obj.GetUID() == child.GetUID() {
			return true
		}
	}
	return false
}

// ListConfigs returns a list of all Secrets and ConfigMaps that are
// referenced in the pod's spec
func (r Owner) ListConfigs(ctx context.Context, namespace string, spec corev1.PodSpec) ([]apis.Object, error) {
	configMaps, secrets := getConfigNamesFromSpec(spec)

	// return error if config resource is not exist
	var configs []apis.Object
	for name := range configMaps {
		key := types.NamespacedName{Namespace: namespace, Name: name}
		configMap := &corev1.ConfigMap{}
		err := r.client.Get(ctx, key, configMap)
		if err != nil {
			return []apis.Object{}, err
		}
		if configMap != nil {
			configs = append(configs, configMap)
		}
	}

	for name := range secrets {
		key := types.NamespacedName{Namespace: namespace, Name: name}
		secret := &corev1.Secret{}
		err := r.client.Get(ctx, key, secret)
		if err != nil {
			return []apis.Object{}, err
		}
		if secret != nil {
			configs = append(configs, secret)
		}
	}

	return configs, nil
}

// getConfigNamesFromSpec parses the owner object and returns two sets,
// the first containing the names of all referenced ConfigMaps,
// the second containing the names of all referenced Secrets
func getConfigNamesFromSpec(spec corev1.PodSpec) (map[string]struct{}, map[string]struct{}) {
	// Create sets for storing the names fo the ConfigMaps/Secrets
	configMaps := make(map[string]struct{})
	secrets := make(map[string]struct{})

	// Iterate over all Volumes and check the VolumeSources for ConfigMaps
	// and Secrets
	for _, vol := range spec.Volumes {
		if cm := vol.VolumeSource.ConfigMap; cm != nil {
			configMaps[cm.Name] = struct{}{}
		}
		if s := vol.VolumeSource.Secret; s != nil {
			secrets[s.SecretName] = struct{}{}
		}
	}

	// Iterate over all Containers and their respective EnvFrom and Env
	// then check the EnvFromSources for ConfigMaps and Secrets
	for _, container := range spec.Containers {
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

// ListConfigsOwnedBy returns a list of all ConfigMaps and Secrets that are
// owned by the instance
func (r Owner) ListConfigsOwnedBy(ctx context.Context, owner apis.Object) ([]apis.Object, error) {
	r.log.Debug("Getting all ConfigMaps and Secrets that are owned by '", owner.GetName(), "'.")
	opts := client.InNamespace(owner.GetNamespace())

	// List all ConfigMaps in the owner's namespace
	configMaps := &corev1.ConfigMapList{}
	err := r.client.List(ctx, opts, configMaps)
	if err != nil {
		return []apis.Object{}, fmt.Errorf("error listing ConfigMaps: %v", err)
	}

	// List all Secrets in the owner's namespace
	secrets := &corev1.SecretList{}
	err = r.client.List(ctx, opts, secrets)
	if err != nil {
		return []apis.Object{}, fmt.Errorf("error listing Secrets: %v", err)
	}

	// Iterate over the ConfigMaps/Secrets and add the ones owned by the
	// owner to the output list configs
	configs := []apis.Object{}
	for _, cm := range configMaps.Items {
		if isOwnedBy(&cm, owner) {
			configs = append(configs, cm.DeepCopy())
		}
	}
	for _, s := range secrets.Items {
		if isOwnedBy(&s, owner) {
			configs = append(configs, s.DeepCopy())
		}
	}

	return configs, nil
}

// isOwnedBy returns true if the child has an owner reference that points to
// the owner object
func isOwnedBy(child, owner apis.Object) bool {
	for _, ref := range child.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}
