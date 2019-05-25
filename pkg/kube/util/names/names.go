package names

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// DeploymentSecretType lists all the types of secrets used in
// the lifecycle of a BOSHDeployment
type DeploymentSecretType int

const (
	// DeploymentSecretTypeManifestWithOps is a manifest that has ops files applied
	DeploymentSecretTypeManifestWithOps DeploymentSecretType = iota
	// DeploymentSecretTypeManifestAndVars is a manifest whose variables have been interpolated
	DeploymentSecretTypeManifestAndVars
	// DeploymentSecretTypeGeneratedVariable is a BOSH variable generated using an ExtendedSecret
	DeploymentSecretTypeGeneratedVariable
	// DeploymentSecretTypeInstanceGroupResolvedProperties is a YAML file containing all properties needed to render an Instance Group
	DeploymentSecretTypeInstanceGroupResolvedProperties
	// DeploymentSecretTypeImplicitVariable is a BOSH variable provided by the user as a Secret
	DeploymentSecretTypeImplicitVariable
	// DeploymentSecretBpmInformation is a YAML file containing the BPM information for one instance group
	DeploymentSecretBpmInformation
)

func (s DeploymentSecretType) String() string {
	return [...]string{
		"with-ops",
		"with-vars",
		"var",
		"ig-resolved",
		"var-implicit",
		"bpm"}[s]
}

// CalculateSecretName generates a Secret name for a given name and a deployment
func CalculateSecretName(secretType DeploymentSecretType, deploymentName, name string) string {
	if name == "" {
		name = secretType.String()
	} else {
		name = fmt.Sprintf("%s-%s", secretType, name)
	}

	nameRegex := regexp.MustCompile("[^-][a-z0-9-]*.[a-z0-9-]*[^-]")
	partRegex := regexp.MustCompile("[a-z0-9-]*")

	deploymentName = partRegex.FindString(strings.Replace(deploymentName, "_", "-", -1))
	variableName := partRegex.FindString(strings.Replace(name, "_", "-", -1))
	secretName := nameRegex.FindString(deploymentName + "." + variableName)

	if len(secretName) > 63 {
		// secret names are limited to 63 characters so we recalculate the name as
		// <name trimmed to 31 characters><md5 hash of name>
		sumHex := md5.Sum([]byte(secretName))
		sum := hex.EncodeToString(sumHex[:])
		secretName = secretName[:63-32] + sum
	}

	return secretName
}

// CalculateEJobOutputSecretPrefixAndName generates a Secret prefix for the output
// of an Extended Job given a name, and calculates the final Secret name,
// given a container name
func CalculateEJobOutputSecretPrefixAndName(secretType DeploymentSecretType, deploymentName string, containerName string, versioned bool) (string, string) {
	prefix := CalculateSecretName(secretType, deploymentName, "")
	finalName := fmt.Sprintf("%s.%s", prefix, containerName)

	if versioned {
		finalName = fmt.Sprintf("%s-v1", finalName)
	}

	return prefix + ".", finalName
}

// GetStatefulSetName gets statefulset name from podName
func GetStatefulSetName(name string) string {
	nameSplit := strings.Split(name, "-")
	nameSplit = nameSplit[0 : len(nameSplit)-1]
	statefulSetName := strings.Join(nameSplit, "-")
	return statefulSetName
}

// GetVersionFromName fetches version from name
func GetVersionFromName(name string, offset int) (int, error) {
	nameSplit := strings.Split(name, "-")
	version := string(nameSplit[len(nameSplit)-offset][1])
	versionInt, err := strconv.Atoi(version)
	if err != nil {
		return versionInt, errors.Wrapf(err, "Atoi failed to convert")
	}
	return versionInt, nil
}

// GetPrefixFromVersionedSecretName gets prefix from versioned secret name
func GetPrefixFromVersionedSecretName(name string) string {
	nameRegex := regexp.MustCompile(`^(\S+)-v\d+$`)
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		prefix := captures[1]
		return prefix
	}

	return ""
}

// GetVersionFromVersionedSecretName gets version from versioned secret name
// return -1 if not find valid version
func GetVersionFromVersionedSecretName(name string) (int, error) {
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

// JobName returns a unique, short name for a given eJob, pod(if exists) combination
// k8s allows 63 chars, but the pod will have -\d{6} appended
func JobName(eJobName, podName string) (string, error) {
	suffix := ""
	if podName == "" {
		suffix = truncate(eJobName, 15)
	} else {
		suffix = fmt.Sprintf("%s-%s", truncate(eJobName, 15), truncate(podName, 15))
	}

	namePrefix := fmt.Sprintf("job-%s", suffix)

	hashID, err := randSuffix(suffix)
	if err != nil {
		return "", errors.Wrap(err, "could not randomize job suffix")
	}
	return fmt.Sprintf("%s-%s", namePrefix, hashID), nil
}

// ServiceName returns a unique, short name for a given instance
func ServiceName(deploymentName, instanceName string, index int) string {
	var serviceName string
	if index == -1 {
		serviceName = fmt.Sprintf("%s-%s", deploymentName, instanceName)
	} else {
		serviceName = fmt.Sprintf("%s-%s-%d", deploymentName, instanceName, index)
	}

	if len(serviceName) > 63 {
		// names are limited to 63 characters so we recalculate the name as
		// <name trimmed to 31 characters>-<md5 hash of name>-headless
		sumHex := md5.Sum([]byte(serviceName))
		sum := hex.EncodeToString(sumHex[:])
		serviceName = fmt.Sprintf("%s-%s", serviceName[:31], sum)
	}

	return serviceName
}

// OrdinalFromPodName returns ordinal from pod name
func OrdinalFromPodName(name string) int {
	podOrdinal := -1
	r := regexp.MustCompile(`(.*)-([0-9]+)$`)

	match := r.FindStringSubmatch(name)
	if len(match) < 3 {
		return podOrdinal
	}
	if i, err := strconv.ParseInt(match[2], 10, 32); err == nil {
		podOrdinal = int(i)
	}
	return podOrdinal
}

func randSuffix(str string) (string, error) {
	randBytes := make([]byte, 16)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", errors.Wrap(err, "could not read rand bytes")
	}

	a := fnv.New64()
	_, err = a.Write([]byte(str + string(randBytes)))
	if err != nil {
		return "", errors.Wrap(err, "could not write hash")
	}

	return hex.EncodeToString(a.Sum(nil)), nil
}

func truncate(name string, max int) string {
	name = strings.Replace(name, "-", "", -1)
	if len(name) > max {
		return name[0:max]
	}
	return name
}
