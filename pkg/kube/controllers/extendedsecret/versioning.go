package extendedsecret

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

var (
	// LabelSecretKind is the label key for secret kind
	LabelSecretKind = fmt.Sprintf("%s/secret-kind", apis.GroupName)
	// LabelVersion is the label key for secret version
	LabelVersion = fmt.Sprintf("%s/secret-version", apis.GroupName)
	// AnnotationSourceDescription is the label key for source description
	AnnotationSourceDescription = fmt.Sprintf("%s/source-description", apis.GroupName)
)

var _ VersionedSecretStore = &VersionedSecretStoreImpl{}

// VersionedSecretStore is the interface to version secrets in Kubernetes
//
// Each update to the secret results in a new persisted version.
// An existing persisted version of a secret cannot be altered or deleted.
// The deletion of a secret will result in the removal of all persisted version of that secret.
//
// The version number is an integer that is incremented with each version of
// the secret, which the greatest number being the current/latest version.
//
// When saving a new secret, a source description is required, which
// should explain the sources of the rendered secret, e.g. the location of
// the Custom Resource Definition that generated it.
type VersionedSecretStore interface {
	Create(ctx context.Context, namespace string, secretName string, secretData map[string]string, labels map[string]string, sourceDescription string) error
	Get(ctx context.Context, namespace string, secretName string, version int) (*corev1.Secret, error)
	Latest(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) (*corev1.Secret, error)
	List(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) ([]corev1.Secret, error)
	VersionCount(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) (int, error)
	Delete(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) error
	Decorate(ctx context.Context, namespace string, secretName string, secretLabels map[string]string, key string, value string) error
}

// VersionedSecretStoreImpl contains the required fields to persist a secret
type VersionedSecretStoreImpl struct {
	client client.Client
}

// NewVersionedSecretStore returns a VersionedSecretStore implementation to be used
// when working with desired secret secrets
func NewVersionedSecretStore(client client.Client) VersionedSecretStoreImpl {
	return VersionedSecretStoreImpl{
		client,
	}
}

// Create creates a new version of the secret from secret data
func (p VersionedSecretStoreImpl) Create(ctx context.Context, namespace string, secretName string, secretData map[string]string, labels map[string]string, sourceDescription string) error {
	currentVersion, err := p.getGreatestVersion(ctx, namespace, secretName, labels)
	if err != nil {
		return err
	}

	version := currentVersion + 1
	labels[LabelVersion] = strconv.Itoa(version)
	labels[LabelSecretKind] = "versionedSecret"

	generatedSecretName, err := generateSecretName(secretName, version)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedSecretName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				AnnotationSourceDescription: sourceDescription,
			},
		},
		StringData: secretData,
	}

	return p.client.Create(ctx, secret)
}

// Get returns a specific version of the secret
func (p VersionedSecretStoreImpl) Get(ctx context.Context, namespace string, deploymentName string, version int) (*corev1.Secret, error) {
	name, err := generateSecretName(deploymentName, version)
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

// Latest returns the latest version of the secret
func (p VersionedSecretStoreImpl) Latest(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) (*corev1.Secret, error) {
	latestVersion, err := p.getGreatestVersion(ctx, namespace, secretName, secretLabels)
	if err != nil {
		return nil, err
	}
	return p.Get(ctx, namespace, secretName, latestVersion)
}

// List returns all versions of the secret
func (p VersionedSecretStoreImpl) List(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) ([]corev1.Secret, error) {
	secrets, err := p.listSecrets(ctx, namespace, secretName, secretLabels)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}

// VersionCount returns the number of versions for this secret
func (p VersionedSecretStoreImpl) VersionCount(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) (int, error) {
	list, err := p.listSecrets(ctx, namespace, secretName, secretLabels)
	if err != nil {
		return 0, err
	}

	return len(list), nil
}

// Decorate adds a label to the latest version of the secret
func (p VersionedSecretStoreImpl) Decorate(ctx context.Context, namespace string, secretName string, secretLabels map[string]string, key string, value string) error {
	version, err := p.getGreatestVersion(ctx, namespace, secretName, secretLabels)
	if err != nil {
		return err
	}

	generatedSecretName, err := generateSecretName(secretName, version)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{}
	if err := p.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: generatedSecretName}, secret); err != nil {
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

// Delete removes all versions of the secret and therefore the
// secret itself.
func (p VersionedSecretStoreImpl) Delete(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) error {
	list, err := p.listSecrets(ctx, namespace, secretName, secretLabels)
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

func (p VersionedSecretStoreImpl) listSecrets(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) ([]corev1.Secret, error) {
	secretLabelsSet := labels.Set(secretLabels)

	secrets := &corev1.SecretList{}
	if err := p.client.List(ctx, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: secretLabelsSet.AsSelector(),
	}, secrets); err != nil {
		return nil, err
	}

	result := []corev1.Secret{}

	nameRegex := regexp.MustCompile(fmt.Sprintf(`^%s-v\d+$`, secretName))
	for _, secret := range secrets.Items {
		if nameRegex.MatchString(secret.Name) {
			result = append(result, secret)
		}
	}

	return result, nil
}

func (p VersionedSecretStoreImpl) getGreatestVersion(ctx context.Context, namespace string, secretName string, secretLabels map[string]string) (int, error) {
	list, err := p.listSecrets(ctx, namespace, secretName, secretLabels)
	if err != nil {
		return -1, err
	}

	var greatestVersion int
	for _, secret := range list {
		version, err := getVersionFromSecretName(secret.GetName())
		if err != nil {
			return 0, err
		}

		if version > greatestVersion {
			greatestVersion = version
		}
	}

	return greatestVersion, nil
}

func generateSecretName(namePrefix string, version int) (string, error) {
	proposedName := fmt.Sprintf("%s-v%d", namePrefix, version)

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

func getVersionFromSecretName(name string) (int, error) {
	nameRegex := regexp.MustCompile(`^\S+-v(\d+)$`)
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		number, err := strconv.Atoi(captures[1])
		if err != nil {
			return -1, errors.Wrapf(err, "invalid secret name %s, it does not end with a version number", name)
		}

		return number, nil
	}

	return -1, fmt.Errorf("invalid secret name %s, it does not match the naming schema", name)
}
