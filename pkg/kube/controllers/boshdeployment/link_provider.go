package boshdeployment

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

func isLinkProviderService(svc *corev1.Service) bool {
	if _, ok := svc.GetAnnotations()[bdv1.AnnotationLinkProviderService]; ok {
		return true
	}

	return false
}

type linkProvider struct {
	Name         string `json:"name"`
	ProviderType string `json:"type"`
}

func newLinkProvider(annotations map[string]string) (linkProvider, error) {
	lp := &linkProvider{}
	if data, ok := annotations[bdv1.AnnotationLinkProvidesKey]; ok {
		err := json.Unmarshal([]byte(data), lp)
		return *lp, err
	}
	return *lp, fmt.Errorf("missing link secrets for providers")
}
