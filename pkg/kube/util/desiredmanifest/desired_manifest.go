package desiredmanifest

import (
	"context"

	"github.com/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
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
func (r *DesiredManifest) DesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*bdm.Manifest, error) {
	// unversioned desired manifest name
	secretName := names.DesiredManifestName(boshDeploymentName, "")

	secret, err := r.versionedSecretStore.Latest(ctx, namespace, secretName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read latest versioned secret %s for bosh deployment %s", secretName, boshDeploymentName)
	}

	manifestData := secret.Data["manifest.yaml"]

	manifest, err := bdm.LoadYAML(manifestData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal manifest from secret %s for boshdeployment %s", secretName, boshDeploymentName)
	}

	return manifest, nil
}
