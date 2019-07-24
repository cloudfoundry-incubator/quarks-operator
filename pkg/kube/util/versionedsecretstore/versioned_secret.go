package versionedsecretstore

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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

// NamePrefix returns the name prefix of a versioned secret name, by removing the
// version suffix /-v\d+/
func NamePrefix(name string) string {
	n := strings.LastIndex(name, "-")
	if n < 1 {
		return ""
	}
	return name[:n]
}

var nameRegex = regexp.MustCompile(`^\S+-v(\d+)$`)

// VersionFromName gets version from versioned secret name
// return -1 if not find valid version
func VersionFromName(name string) (int, error) {
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		number, err := strconv.Atoi(captures[1])
		if err != nil {
			return -1, errors.Wrapf(err, "invalid secret name %s, it does not end with a version number", name)
		}

		return number, nil
	}

	return -1, errors.Errorf("invalid secret name %s, it does not match the naming schema", name)
}

// ContainsSecretName checks a list of secret names for our secret's name
// while ignoring the versions
func ContainsSecretName(names []string, name string) bool {
	unversioned := NamePrefix(name)
	if unversioned == "" {
		return false
	}
	for _, k := range names {
		if strings.Contains(k, unversioned) {
			return true
		}
	}
	return false
}

// IsInitialVersion returns true if it's a v1 secret
func IsInitialVersion(secret corev1.Secret) bool {
	version, ok := secret.Labels[LabelVersion]
	if !ok {
		return false
	}

	return version == "1"
}

// Version returns the versioned secrets version from the labels
func Version(secret corev1.Secret) (int, error) {
	version, ok := secret.Labels[LabelVersion]
	if !ok {
		return -1, errors.Errorf("secret '%s' has no version label", secret.Name)
	}

	number, err := strconv.Atoi(version)
	if err != nil {
		return -1, errors.Wrapf(err, "invalid secret version '%s', is not a number", version)
	}

	return number, nil
}
