package names

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

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

// GetPrefixFromVersionedSecretName gets prefix from versioned secret
func GetPrefixFromVersionedSecretName(name string) string {
	nameRegex := regexp.MustCompile(`^(\S+)-v\d+$`)
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		prefix := captures[1]
		return prefix
	}

	return ""
}

// JobName returns a unique, short name for a given extJob, pod(if exists) combination
// k8s allows 63 chars, but the pod will have -\d{6} appended
// IDEA: maybe use pod.Uid instead of rand
func JobName(extJobName, podName string) (string, error) {
	suffix := ""
	if podName == "" {
		suffix = truncate(extJobName, 15)
	} else {
		suffix = fmt.Sprintf("%s-%s", truncate(extJobName, 15), truncate(podName, 15))
	}

	namePrefix := fmt.Sprintf("job-%s", suffix)

	hashID, err := randSuffix(suffix)
	if err != nil {
		return "", errors.Wrap(err, "could not randomize job suffix")
	}
	return fmt.Sprintf("%s-%s", namePrefix, hashID), nil
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
