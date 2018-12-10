package secrets

import (
	"fmt"
	"regexp"
	"strconv"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const manifestKeyName = "manifest"

type ManifestPersister interface {
	CreateDesiredManifestSecret(manifest manifest.Manifest) (*corev1.Secret, error)
	DeleteManifestSecrets() error
	// deleting a desired manifest secret
	// decorating the secret with key/value labels
	ListAllVersions() ([]corev1.Secret, error)
	RetrieveVersion(version int) (*corev1.Secret, error)
	RetrieveLatestVersion() (*corev1.Secret, error)
}

type ManifestPersisterImpl struct {
	client         kubernetes.Interface
	namespace      string
	deploymentName string
}

func NewManifestPersister(client kubernetes.Interface, namespace string, deploymentName string) *ManifestPersisterImpl {
	return &ManifestPersisterImpl{
		client,
		namespace,
		deploymentName,
	}
}

func (p *ManifestPersisterImpl) CreateDesiredManifestSecret(manifest manifest.Manifest) (*corev1.Secret, error) {
	// First check if a secret of the same type(deployment-deploymentName) exists
	version, err := getGreatestVersion(p)
	if err != nil {
		return nil, err
	}

	secretName, err := secretName(p, version+1)
	if err != nil {
		return nil, err
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	return p.client.CoreV1().Secrets(p.namespace).Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: p.namespace,
		},
		Data: map[string][]byte{
			manifestKeyName: data,
		},
	})
}

func (p *ManifestPersisterImpl) RetrieveVersion(version int) (*corev1.Secret, error) {
	list, err := p.ListAllVersions()
	if err != nil {
		return nil, err
	}

	for _, secret := range list {
		ver, err := getVersionFromSecretName(p, secret.Name)
		if err != nil {
			return nil, err
		}

		if ver == version {
			return &secret, nil
		}
	}
	return nil, fmt.Errorf("unable to find the requested version: %d", version)
}

func (p *ManifestPersisterImpl) RetrieveLatestVersion() (*corev1.Secret, error) {
	latestVersion, err := getGreatestVersion(p)
	if err != nil {
		return nil, err
	}
	return p.RetrieveVersion(latestVersion)
}

func (p *ManifestPersisterImpl) ListAllVersions() ([]corev1.Secret, error) {
	listResp, err := p.client.CoreV1().Secrets(p.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := []corev1.Secret{}

	nameRegex := regexp.MustCompile(fmt.Sprintf(`^deployment-%s-\d+$`, p.deploymentName))
	for _, secret := range listResp.Items {
		if nameRegex.MatchString(secret.Name) {
			result = append(result, secret)
		}
	}
	return result, nil
}

func (p *ManifestPersisterImpl) DeleteManifestSecrets() error {
	list, err := p.ListAllVersions()
	if err != nil {
		return err
	}

	for _, secret := range list {
		err := p.client.CoreV1().Secrets(p.namespace).Delete(secret.Name, &metav1.DeleteOptions{})

		if err != nil {
			return err
		}
	}
	return nil
}

func getGreatestVersion(p *ManifestPersisterImpl) (int, error) {
	list, err := p.ListAllVersions()
	if err != nil {
		return -1, err
	}

	var greatestVersion int
	for _, secret := range list {
		version, err := getVersionFromSecretName(p, secret.Name)
		if err != nil {
			return 0, err
		}

		if version > greatestVersion {
			greatestVersion = version
		}
	}

	return greatestVersion, nil
}

func secretName(p *ManifestPersisterImpl, version int) (string, error) {
	proposedName := fmt.Sprintf("deployment-%s-%d", p.deploymentName, version)

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

func getVersionFromSecretName(p *ManifestPersisterImpl, name string) (int, error) {
	nameRegex := regexp.MustCompile(fmt.Sprintf(`^deployment-%s-(\d+)$`, p.deploymentName))
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		number, err := strconv.Atoi(captures[1])
		if err != nil {
			return -1, errors.Wrapf(err, "invalid secret name %s, it does not end with a version number", name)
		}

		return number, nil
	}

	return -1, fmt.Errorf("invalid secret name %s, it does not match the naming schema", name)
}
