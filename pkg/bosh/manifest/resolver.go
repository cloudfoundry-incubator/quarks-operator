package manifest

import (
	"context"
	"fmt"

	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeploymentcontroller/v1alpha1"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolver resolves references from CRD to a BOSH manifest
type Resolver interface {
	ResolveCRD(bdc.BOSHDeploymentSpec, string) (*Manifest, error)
}

// ResolverImpl implements Resolver interface
type ResolverImpl struct {
	client       client.Client
	interpolator Interpolator
}

// NewResolver constructs a resolver
func NewResolver(client client.Client, interpolator Interpolator) *ResolverImpl {
	return &ResolverImpl{client: client, interpolator: interpolator}
}

// ResolveCRD returns manifest referenced by our CRD
func (r *ResolverImpl) ResolveCRD(spec bdc.BOSHDeploymentSpec, namespace string) (*Manifest, error) {
	manifest := &Manifest{}

	// TODO for now we only support config map ref
	ref := spec.ManifestRef

	config := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ref, Namespace: namespace}, config)
	if err != nil {
		return manifest, errors.Wrapf(err, "Failed to retrieve configmap '%s/%s' via client.Get", namespace, ref)
	}

	// unmarshal manifest.data into bosh deployment manifest...
	// TODO re-use LoadManifest() from fissile
	m, ok := config.Data["manifest"]
	if !ok {
		return manifest, fmt.Errorf("configmap doesn't contain manifest key")
	}

	// unmarshal ops.data into bosh ops if exist
	opsConfig := &corev1.ConfigMap{}
	opsRef := spec.OpsRef
	if opsRef == "" {
		err = yaml.Unmarshal([]byte(m), manifest)
		return manifest, err
	}

	err = r.client.Get(context.TODO(), types.NamespacedName{Name: opsRef, Namespace: namespace}, opsConfig)
	if err != nil {
		return manifest, errors.Wrapf(err, "Failed to retrieve configmap '%s/%s' via client.Get", namespace, opsRef)
	}

	opsData, ok := opsConfig.Data["ops"]
	if !ok {
		return manifest, fmt.Errorf("configmap doesn't contain ops key")
	}

	err = r.interpolator.BuildOps([]byte(opsData))
	if err != nil {
		return manifest, errors.Wrapf(err, "Failed to build ops: %#v", opsData)
	}

	bytes, err := r.interpolator.Interpolate([]byte(m))
	if err != nil {
		return manifest, errors.Wrapf(err, "Failed to interpolate %#v by %#v", m, opsData)
	}

	err = yaml.Unmarshal(bytes, manifest)

	return manifest, err
}
