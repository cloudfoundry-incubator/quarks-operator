package boshdns_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
)

const (
	invalid = `---
addons:
- name: bosh-dns-aliases
  jobs:
  - name: bosh-dns-aliases
    release: bosh-dns-aliases
    properties:
      aliases: {}
instance_groups:
  - name: component1
    instances: 1
    jobs:
    - name: job1
      properties:
        url: https://uaa.service.cf.internal:8443/test/
variables:
  - name: router_ca
    type: certificate
    options:
      is_ca: true
      common_name: uaa.service.cf.internal
`
	nodns = `---
addons:
- name: unknown-addon
  jobs: []
instance_groups:
  - name: component1
    instances: 1
    jobs:
    - name: job1
      properties:
        url: https://uaa.service.cf.internal:8443/test/
variables:
  - name: router_ca
    type: certificate
    options:
      is_ca: true
      common_name: uaa.service.cf.internal
`
	valid = `---
addons:
- name: bosh-dns-aliases
  jobs:
  - name: bosh-dns-aliases
    release: bosh-dns-aliases
    properties:
      aliases:
      - domain: 'uaa.service.cf.internal'
        targets:
        - query: '_'
          instance_group: singleton-uaa
          deployment: cf
          network: default
          domain: bosh
instance_groups:
  - name: component1
    instances: 1
    jobs:
    - name: job1
      properties:
        url: https://uaa.service.cf.internal:8443/test/
variables:
  - name: router_ca
    type: certificate
    options:
      is_ca: true
      common_name: uaa.service.cf.internal
`
)

// Interface switches between the two implementations of boshdns
var _ = Describe("Interface", func() {
	Context("Validate", func() {
		Context("when manifest is valid", func() {
			It("does not err", func() {
				m, err := manifest.LoadYAML([]byte(valid))
				Expect(err).NotTo(HaveOccurred())
				err = boshdns.Validate(*m)
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("when manifest is invalid", func() {
			It("returns an error", func() {
				m, err := manifest.LoadYAML([]byte(invalid))
				Expect(err).NotTo(HaveOccurred())
				err = boshdns.Validate(*m)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when manifest has no dns addon", func() {
			It("returns no error", func() {
				m, err := manifest.LoadYAML([]byte(nodns))
				Expect(err).NotTo(HaveOccurred())
				err = boshdns.Validate(*m)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("DNSSetting", func() {
		It("returns clusterDNSFirst when boshdns addon is not present", func() {
			m, err := manifest.LoadYAML([]byte(nodns))
			Expect(err).NotTo(HaveOccurred())

			policy, config, err := boshdns.DNSSetting(*m, "1.2.3.5", "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(policy).To(Equal(corev1.DNSClusterFirst))
			Expect(config).To(BeNil())
		})

		It("returns DNSNone when bosh dns addon is present", func() {
			m, err := manifest.LoadYAML([]byte(valid))
			Expect(err).NotTo(HaveOccurred())

			policy, config, err := boshdns.DNSSetting(*m, "1.2.3.5", "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(policy).To(Equal(corev1.DNSNone))
			Expect(config).NotTo(BeNil())
		})
	})

	Context("CustomDNSSetting", func() {
		It("returns custom dns", func() {
			policy, config := boshdns.CustomDNSSetting("1.2.3.5", "default")
			Expect(policy).To(Equal(corev1.DNSNone))
			Expect(config).NotTo(BeNil())
			Expect(config.Nameservers).To(Equal([]string{"1.2.3.5"}))
			Expect(config.Searches).To(ContainElements("default.svc.", "svc.", ""))
		})
	})
})
