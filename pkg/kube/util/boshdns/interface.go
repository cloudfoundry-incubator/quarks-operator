package boshdns

import (
	"context"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
)

// DomainNameService abstraction.
type DomainNameService interface {
	// DNSSetting get the DNS settings for POD.
	DNSSetting(namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error)

	// Apply a DNS server to the given namespace, if required.
	Apply(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error
}

// New returns the DNS service management struct
func New(m bdm.Manifest) (DomainNameService, error) {
	for _, addon := range m.AddOns {
		if addon.Name == bdm.BoshDNSAddOnName {
			var err error
			dns, err := NewBoshDomainNameService(addon, m.InstanceGroups)
			if err != nil {
				return nil, errors.Wrapf(err, "error loading BOSH DNS configuration")
			}
			return dns, nil
		}
	}

	return NewSimpleDomainNameService(), nil
}

// Validate that all job properties of the addon section can be decoded
func Validate(m bdm.Manifest) error {
	_, err := New(m)
	return err
}
