// Package desiredmanifest retrieves the latest desired manifest
package desiredmanifest

import (
	"context"

	"github.com/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

var (
	// Name is the name of the container that
	// performs variable interpolation for a manifest. It's also part of
	// the output secret's name
	Name = "desired-manifest"
)

// DesiredManifest resolves references from bdpl CRD to a BOSH manifest
type DesiredManifest struct {
	client               client.Client
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// NewDesiredManifest constructs a resolver
func NewDesiredManifest(client client.Client) *DesiredManifest {
	return &DesiredManifest{
		client:               client,
		versionedSecretStore: versionedsecretstore.NewVersionedSecretStore(client),
	}
}

// DesiredManifest reads the versioned secret created by the variable interpolation job
// and unmarshals it into a Manifest object
func (r *DesiredManifest) DesiredManifest(ctx context.Context, namespace string) (*bdm.Manifest, error) {
	secret, err := r.versionedSecretStore.Latest(ctx, namespace, Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read latest versioned secret %s for bosh deployment in %s", Name, namespace)
	}

	manifestData := secret.Data["manifest.yaml"]

	manifest, err := bdm.LoadYAML(manifestData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal manifest from secret %s for boshdeployment in %s", Name, namespace)
	}

	return manifest, nil
}
