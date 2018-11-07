package manifest

import (
	"context"
	"fmt"

	fissile "code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolver resolves references from CRD to a BOSH manifest
type Resolver interface {
	ResolveCRD(fissile.BOSHDeploymentSpec, string) (*Manifest, error)
}

// ResolverImpl implements Resolver interface
type ResolverImpl struct {
	client client.Client
}

// NewResolver constructs a resolver
func NewResolver(client client.Client) *ResolverImpl {
	return &ResolverImpl{client: client}
}

// ResolveCRD returns manifest referenced by our CRD
func (r *ResolverImpl) ResolveCRD(spec fissile.BOSHDeploymentSpec, namespace string) (*Manifest, error) {
	manifest := &Manifest{}

	// TODO for now we only support config map ref
	ref := spec.ManifestRef

	config := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ref, Namespace: namespace}, config)
	if err != nil {
		return manifest, errors.Wrap(err, "failed to retrieve via client.Get")
	}

	// unmarshal manifest.data into bosh deployment manifest...
	// TODO re-use LoadManifest() from fisisle
	m, ok := config.Data["manifest"]
	if !ok {
		return manifest, fmt.Errorf("configmap doesn't contain manifest key")
	}
	err = yaml.Unmarshal([]byte(m), manifest)

	return manifest, err
}
