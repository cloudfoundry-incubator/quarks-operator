package boshdns_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
})
