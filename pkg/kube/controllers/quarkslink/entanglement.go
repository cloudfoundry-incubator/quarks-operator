package quarkslink

import (
	"encoding/json"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

var (
	// DeploymentKey is the key to retrieve the name of the deployment,
	// which provides the variables for the pod
	DeploymentKey = fmt.Sprintf("%s/deployment", apis.GroupName)
	// ConsumesKey is the key for identifying the provider to be consumed, in the
	// format of 'type.job'
	ConsumesKey = fmt.Sprintf("%s/consumes", apis.GroupName)
)

func validEntanglement(annotations map[string]string) bool {
	if annotations[DeploymentKey] != "" && annotations[ConsumesKey] != "" {
		return validLinksJSON(annotations[ConsumesKey])
	}
	return false
}

func validLinksJSON(value string) bool {
	links, err := newLinks(value)
	if err != nil {
		return false
	}
	if len(links) == 0 {
		return false
	}

	for _, link := range links {
		if link.Name == "" || link.LinkType == "" {
			return false
		}
	}
	return true
}

func newLinks(value string) (links, error) {
	l := &links{}
	err := json.Unmarshal([]byte(value), l)
	return *l, err
}

type link struct {
	Name       string `json:"name"`
	LinkType   string `json:"type"`
	secretName string
}

func (l link) String() string {
	return fmt.Sprintf("%s.%s", l.Name, l.LinkType)
}

type links []link

type entanglement struct {
	deployment string
	consumes   string
	links      links
}

func newEntanglement(obj map[string]string) entanglement {
	links, _ := newLinks(obj[ConsumesKey])
	e := entanglement{
		deployment: obj[DeploymentKey],
		consumes:   obj[ConsumesKey],
		links:      links,
	}
	return e
}

func (e entanglement) find(secret corev1.Secret) (link, bool) {
	// secret has a deployment label
	entanglementDeployment, found := secret.Labels[manifest.LabelDeploymentName]
	if !found {
		return link{}, false
	}

	// deployment label matches entanglements'
	if entanglementDeployment != e.deployment {
		return link{}, false
	}

	// secret name is a valid quarks link name and matches deployment
	var regex = regexp.MustCompile(fmt.Sprintf("^link-%s-[a-z0-9-]*$", e.deployment))
	if !regex.MatchString(secret.Name) {
		return link{}, false
	}

	// secret contains one of the requested properties
	for _, link := range e.links {
		if _, found := secret.Data[link.String()]; found {
			return link, true
		}
	}
	return link{}, false
}
