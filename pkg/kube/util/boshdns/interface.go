package boshdns

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
)

// DomainNameService abstraction.
type DomainNameService interface {
	// Apply a DNS server to the given namespace, if required.
	Apply(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error
}

// New returns the DNS service management struct
func New(m bdm.Manifest) (DomainNameService, error) {
	dns := NewBoshDomainNameService(m.InstanceGroups)
	found := false
	for index, addon := range m.AddOns {
		for _, job := range addon.Jobs {
			if job.Release == bdm.BoshDNSAddOnName || job.Release == bdm.BOSHDNSAliasesAddOnName {
				found = true
				if err := dns.Add(m.AddOns[index]); err != nil {
					return nil, errors.Wrapf(err, "error loading BOSH DNS configuration")
				}
			}
		}
	}
	if found {
		return dns, nil
	}

	return NewSimpleDomainNameService(), nil
}

// Validate that all job properties of the addon section can be decoded
func Validate(m bdm.Manifest) error {
	_, err := New(m)
	return err
}

// CustomDNSSetting sets the pod dns policy.
func CustomDNSSetting(serviceIP, namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig) {
	ndots := "5"
	return corev1.DNSNone, &corev1.PodDNSConfig{
		Nameservers: []string{serviceIP},
		Searches: []string{
			fmt.Sprintf("%s.svc.%s", namespace, clusterDomain),
			fmt.Sprintf("svc.%s", clusterDomain),
			clusterDomain,
		},
		Options: []corev1.PodDNSConfigOption{{Name: "ndots", Value: &ndots}},
	}
}

// DNSSetting sets the pod dns policy.
func DNSSetting(m bdm.Manifest, serviceIP, namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error) {
	index := HasBoshDNSAddOn(m)
	if index != -1 {
		if serviceIP == "" {
			return corev1.DNSNone, nil, errors.New("BoshDomainNameService: DNSSetting called before Apply")
		}
		p, c := CustomDNSSetting(serviceIP, namespace)
		return p, c, nil
	}

	return corev1.DNSClusterFirst, nil, nil
}

// HasBoshDNSAddOn checks if the manifest has bosh dns addon
func HasBoshDNSAddOn(m bdm.Manifest) int {
	index := -1
	for index, addon := range m.AddOns {
		for _, job := range addon.Jobs {
			if job.Release == bdm.BoshDNSAddOnName || job.Release == bdm.BOSHDNSAliasesAddOnName {
				return index
			}
		}
	}

	return index
}
