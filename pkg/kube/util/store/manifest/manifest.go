package manifeststore

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

var (
	manifestKeyName      = "manifest"
	LabelVersionName     = fmt.Sprintf("%s/version", apis.GroupName)
	LabelDeploymentName  = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	AnnotationSourceName = fmt.Sprintf("%s/source-description", apis.GroupName)
)

var _ Store = &StoreImpl{}

// Store is the interface to persist manifests in Kubernetes
//
// It uses Kubernetes secrets to persist a manifest. There can only be one
// manifest per deployment. Each update to the manifest results in a new
// persisted version. An existing persisted version of a manifest cannot be
// altered or deleted. The deletion of a manifest will result in the removal
// of all persisted version of that manifest.
//
// The version number is an integer that is incremented with each version of
// the manifest, which the greatest number being the current/latest version.
//
// When persisting a new manifest, a source description is required, which
// should explain the sources of the rendered manifest, e.g. the location of
// the Custom Resource Definition that generated it.
type Store interface {
	Save(ctx context.Context, namespace string, deploymentName string, manifest bdm.Manifest, sourceDescription string) error
	RetrieveVersionSecret(ctx context.Context, namespace string, deploymentName string, version int) (*corev1.Secret, error)
	Find(ctx context.Context, namespace string, deploymentName string, version int) (*bdm.Manifest, error)
	Latest(ctx context.Context, namespace string, deploymentName string) (*bdm.Manifest, error)
	List(ctx context.Context, namespace string, deploymentName string) ([]*bdm.Manifest, error)
	VersionCount(ctx context.Context, namespace string, deploymentName string) (int, error)
	Delete(ctx context.Context, namespace string, deploymentName string) error
	Decorate(ctx context.Context, namespace string, deploymentName string, key string, value string) error
}

// StoreImpl contains the required fields to persist a manifest
type StoreImpl struct {
	client client.Client
}

// NewStore returns a Store implementation to be used
// when working with desired manifest secrets
func NewStore(client client.Client) StoreImpl {
	return StoreImpl{
		client,
	}
}

// Save creates a new version of the manifest if it already exists,
// or a first one it not. A source description should explain the sources of
// the rendered manifest, e.g. the location of the CRD that generated it
func (p StoreImpl) Save(ctx context.Context, namespace string, deploymentName string, manifest bdm.Manifest, sourceDescription string) error {
	currentVersion, err := p.getGreatestVersion(ctx, namespace, deploymentName)
	if err != nil {
		return err
	}

	version := currentVersion + 1

	secretName, err := secretName(deploymentName, version)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				LabelDeploymentName: deploymentName,
				LabelVersionName:    strconv.Itoa(version),
			},
			Annotations: map[string]string{
				AnnotationSourceName: sourceDescription,
			},
		},
		Data: map[string][]byte{
			manifestKeyName: data,
		},
	}

	return p.client.Create(ctx, secret)
}

// RetrieveVersionSecret retrieves the k8s secret containing a manifest version
func (p StoreImpl) RetrieveVersionSecret(ctx context.Context, namespace string, deploymentName string, version int) (*corev1.Secret, error) {
	name, err := secretName(deploymentName, version)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	err = p.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// Find returns a specific version of the manifest
func (p StoreImpl) Find(ctx context.Context, namespace string, deploymentName string, version int) (*bdm.Manifest, error) {
	secret, err := p.RetrieveVersionSecret(ctx, namespace, deploymentName, version)
	if err != nil {
		return nil, err
	}
	return extractManifest(*secret)
}

// Latest returns the latest version of the manifest
func (p StoreImpl) Latest(ctx context.Context, namespace string, deploymentName string) (*bdm.Manifest, error) {
	latestVersion, err := p.getGreatestVersion(ctx, namespace, deploymentName)
	if err != nil {
		return nil, err
	}
	return p.Find(ctx, namespace, deploymentName, latestVersion)
}

// List returns all versions of the manifest
func (p StoreImpl) List(ctx context.Context, namespace string, deploymentName string) ([]*bdm.Manifest, error) {
	secrets, err := p.listSecrets(ctx, namespace, deploymentName)
	if err != nil {
		return nil, err
	}

	manifests := make([]*bdm.Manifest, 0, len(secrets))
	for _, s := range secrets {
		m, err := extractManifest(s)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// VersionCount returns the number of versions for this manifest
func (p StoreImpl) VersionCount(ctx context.Context, namespace string, deploymentName string) (int, error) {
	list, err := p.listSecrets(ctx, namespace, deploymentName)
	if err != nil {
		return 0, err
	}

	return len(list), nil
}

// Decorate adds a label to the latest version of the manifest
func (p StoreImpl) Decorate(ctx context.Context, namespace string, deploymentName string, key string, value string) error {
	version, err := p.getGreatestVersion(ctx, namespace, deploymentName)
	if err != nil {
		return err
	}

	secretName, err := secretName(deploymentName, version)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{}
	if err := p.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret); err != nil {
		return err
	}

	labels := secret.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[key] = value
	secret.SetLabels(labels)

	return p.client.Update(ctx, secret)
}

// Delete removes all versions of the manifest and therefore the
// manifest itself.
func (p StoreImpl) Delete(ctx context.Context, namespace string, deploymentName string) error {
	list, err := p.listSecrets(ctx, namespace, deploymentName)
	if err != nil {
		return err
	}

	for _, secret := range list {
		if err := p.client.Delete(ctx, &secret); err != nil {
			return err
		}
	}

	return nil
}

func (p StoreImpl) listSecrets(ctx context.Context, namespace string, deploymentName string) ([]corev1.Secret, error) {
	// TODO list by label selector
	secrets := &corev1.SecretList{}
	if err := p.client.List(ctx, client.InNamespace(namespace), secrets); err != nil {
		return nil, err
	}

	result := []corev1.Secret{}

	nameRegex := regexp.MustCompile(fmt.Sprintf(`^deployment-%s-\d+$`, deploymentName))
	for _, secret := range secrets.Items {
		if nameRegex.MatchString(secret.Name) {
			result = append(result, secret)
		}
	}

	return result, nil
}

func (p StoreImpl) getGreatestVersion(ctx context.Context, namespace string, deploymentName string) (int, error) {
	list, err := p.listSecrets(ctx, namespace, deploymentName)
	if err != nil {
		return -1, err
	}

	var greatestVersion int
	for _, secret := range list {
		version, err := getVersionFromSecretName(deploymentName, secret.Name)
		if err != nil {
			return 0, err
		}

		if version > greatestVersion {
			greatestVersion = version
		}
	}

	return greatestVersion, nil
}

func extractManifest(s corev1.Secret) (*bdm.Manifest, error) {

	data, found := s.Data[manifestKeyName]
	if !found {
		return nil, fmt.Errorf("failed to retrieve manifest data from secret '%s.%s'", s.Name, manifestKeyName)
	}

	var manifest bdm.Manifest
	err := yaml.Unmarshal([]byte(data), &manifest)
	return &manifest, err
}

func secretName(deploymentName string, version int) (string, error) {
	proposedName := fmt.Sprintf("deployment-%s-%d", deploymentName, version)

	// Check for Kubernetes name requirements (length)
	const maxChars = 253
	if len(proposedName) > maxChars {
		return "", fmt.Errorf("secret name exceeds maximum number of allowed characters (actual=%d, allowed=%d)", len(proposedName), maxChars)
	}

	// Check for Kubernetes name requirements (characters)
	if re := regexp.MustCompile(`[^a-z0-9.-]`); re.MatchString(proposedName) {
		return "", fmt.Errorf("secret name contains invalid characters, only lower case, dot and dash are allowed")
	}

	return proposedName, nil
}

func getVersionFromSecretName(deploymentName string, name string) (int, error) {
	nameRegex := regexp.MustCompile(fmt.Sprintf(`^deployment-%s-(\d+)$`, deploymentName))
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		number, err := strconv.Atoi(captures[1])
		if err != nil {
			return -1, errors.Wrapf(err, "invalid secret name %s, it does not end with a version number", name)
		}

		return number, nil
	}

	return -1, fmt.Errorf("invalid secret name %s, it does not match the naming schema", name)
}
