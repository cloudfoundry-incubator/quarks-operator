package manifest

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// DesiredManifest unmarshals desired manifest from the manifest secret
type DesiredManifest interface {
	DesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*bdm.Manifest, error)
	InjectClient(client.Client)
}

type service struct {
	client               client.Client
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// NewDesiredManifest constructs a resolver
func NewDesiredManifest() DesiredManifest {
	return &service{}
}

// DesiredManifest reads the versioned secret created by the variable interpolation job
// and unmarshals it into a Manifest object
func (s *service) DesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*bdm.Manifest, error) {
	// unversioned desired manifest name
	secretName := names.DesiredManifestName(boshDeploymentName, "")

	secret, err := s.versionedSecretStore.Latest(ctx, namespace, secretName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read latest versioned secret %s for bosh deployment %s", secretName, boshDeploymentName)
	}

	manifestData := secret.Data["manifest.yaml"]

	manifest, err := bdm.LoadYAML(manifestData)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal manifest from secret %s for boshdeployment %s", secretName, boshDeploymentName)
	}

	return manifest, nil
}

// InjectClient adds the k8s client to the service
func (s *service) InjectClient(client client.Client) {
	s.client = client
	s.versionedSecretStore = versionedsecretstore.NewVersionedSecretStore(client)
}
