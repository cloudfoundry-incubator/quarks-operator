package manifest

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// LatestVersion returns the latest desired manifest version. It returns the initial
// version "1" if no desired manifest version is found.
func (r *Resolver) LatestVersion(ctx context.Context, namespace string, manifestName string) string {
	secretName := names.DesiredManifestName(manifestName, "")
	secret, err := r.versionedSecretStore.Latest(ctx, namespace, secretName)
	if err != nil {
		return "1"
	}
	version, err := versionedsecretstore.Version(*secret)
	if err != nil {
		return "1"
	}
	return strconv.Itoa(version)
}

// ReadDesiredManifest reads the versioned secret created by the variable interpolation job
// and unmarshals it into a Manifest object
func (r *Resolver) ReadDesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*Manifest, error) {
	// unversioned desired manifest name
	secretName := names.DesiredManifestName(boshDeploymentName, "")

	secret, err := r.versionedSecretStore.Latest(ctx, namespace, secretName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read versioned secret for desired manifest")
	}

	manifestData := secret.Data["manifest.yaml"]

	manifest := &Manifest{}
	err = yaml.Unmarshal(manifestData, manifest)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal manifest from secret '%s'", secretName)
	}

	return manifest, nil
}
