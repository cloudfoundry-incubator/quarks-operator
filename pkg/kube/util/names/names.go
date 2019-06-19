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

// DesiredManifestPrefix returns the prefix of the desired manifest's name:
func DesiredManifestPrefix(deploymentName string) string {
	return Sanitize(deploymentName) + "."
}

// DesiredManifestName returns the versioned name of the desired manifest
// secret, e.g. 'test.desired-manifest-v1'
func DesiredManifestName(deploymentName string, version string) string {
	finalName := DesiredManifestPrefix(deploymentName) + "desired-manifest"
	if version != "" {
		finalName = fmt.Sprintf("%s-v%s", finalName, version)
	}

	return finalName
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

	return truncateMD5(secretName)
}

// CalculateIGSecretName returns the name of a k8s secret:
// `<deployment-name>.<secretType>.<instance-group>-v<version>` secret.
//
// These secrets are created by ExtendedJob and mounted on containers, e.g.
// for the template rendering.
func CalculateIGSecretName(secretType DeploymentSecretType, deploymentName string, igName string, version string) string {
	prefix := CalculateIGSecretPrefix(secretType, deploymentName)
	finalName := prefix + Sanitize(igName)

	if version != "" {
		finalName = fmt.Sprintf("%s-v%s", finalName, version)
	}

	return finalName
}

// CalculateIGSecretPrefix returns the prefix used for our k8s secrets:
// `<deployment-name>.<secretType>.
func CalculateIGSecretPrefix(secretType DeploymentSecretType, deploymentName string) string {
	return CalculateSecretName(secretType, deploymentName, "") + "."
}

var allowedKubeChars = regexp.MustCompile("[^-a-z0-9]*")

// Sanitize produces valid k8s names, i.e. for containers: [a-z0-9]([-a-z0-9]*[a-z0-9])?
func Sanitize(name string) string {
	name = strings.Replace(name, "_", "-", -1)
	name = strings.ToLower(name)
	name = allowedKubeChars.ReplaceAllLiteralString(name, "")
	name = strings.TrimPrefix(name, "-")
	name = strings.TrimSuffix(name, "-")
	name = truncateMD5(name)
	return name
}

func truncateMD5(s string) string {
	if len(s) > 63 {
		// names are limited to 63 characters so we recalculate the name as
		// <name trimmed to 31 characters>-<md5 hash of name>
		sumHex := md5.Sum([]byte(s))
		sum := hex.EncodeToString(sumHex[:])
		s = s[:63-32] + sum
	}
	return s
}

// GetStatefulSetName gets statefulset name from podName
func GetStatefulSetName(name string) string {
	nameSplit := strings.Split(name, "-")
	nameSplit = nameSplit[0 : len(nameSplit)-1]
	statefulSetName := strings.Join(nameSplit, "-")
	return statefulSetName
}

// JobName returns a unique, short name for a given eJob, pod(if exists) combination
// k8s allows 63 chars, but the pod will have -\d{6} appended
// So we return max 56 chars: name19(-podname19)-suffix16
func JobName(eJobName, podName string) (string, error) {
	name := ""
	if podName == "" {
		name = truncate(eJobName, 39)
	} else {
		name = fmt.Sprintf("%s-%s", truncate(eJobName, 19), truncate(podName, 19))
	}

	hashID, err := randSuffix(name)
	if err != nil {
		return "", errors.Wrap(err, "could not randomize job suffix")
	}
	return fmt.Sprintf("%s-%s", name, hashID), nil
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
	if len(name) > max {
		return name[0:max]
	}
	return name
}
