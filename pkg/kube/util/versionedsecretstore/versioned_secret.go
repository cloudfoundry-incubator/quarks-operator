package versionedsecretstore

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// IsVersionedSecret returns true if the secret has a label identifying it as versioned secret
func IsVersionedSecret(secret corev1.Secret) bool {
	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		return false
	}

	if kind, ok := secretLabels[LabelSecretKind]; ok && kind == VersionSecretKind {
		return true
	}

	return false
}

// UnversionedName returns the unversioned name of a secret by removing the /-v\d+/ suffix
func UnversionedName(name string) (string, error) {
	n := strings.LastIndex(name, "-")
	if n < 1 {
		return "", fmt.Errorf("failed to parse versioned secret: %s", name)
	}
	return name[:n], nil

}

// ContainsSecretName checks a list of secret names for our secret's name
// while ignoring the versions
func ContainsSecretName(names []string, name string) (bool, error) {
	unversioned, err := UnversionedName(name)
	if err != nil {
		return false, err
	}
	for _, k := range names {
		if strings.Index(k, unversioned) > -1 {
			return true, nil
		}
	}
	return false, nil
}

// IsInitialVersion returns true if it's a v1 secret
func IsInitialVersion(secret corev1.Secret) bool {
	version, ok := secret.Labels[LabelVersion]
	if !ok {
		return false
	}

	return version == "1"
}
