package integration_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("BOSH DNS coredns", func() {

	const aliasAddon = `
{
  "name": "bosh-dns-aliases",
  "jobs": [
    {
      "name": "bosh-dns-aliases",
      "release": "bosh-dns-aliases",
      "properties": {
        "aliases": [
          {
            "domain": "uaa.service.cf.internal",
            "targets": [
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "uaa",
                "network": "default",
                "query": "*"
              }
            ]
          }
        ]
      }
    }
  ]
}
`

	const handlerAddon = `
{
  "name": "bosh-dns-handler",
  "jobs": [
    {
      "name": "bosh-dns-handler",
      "release": "fake-release",
      "properties": {
        "handlers": [
          {
            "domain": "corp.intranet.local.",
            "source": {
              "recursors": [ "10.0.0.2", "127.0.0.1" ],
              "type": "dns"
            }
          }
        ]
      }
    }
  ]
}
`

	var tearDowns []machine.TearDownFunc

	loadAddOn := func(addon string) *bdm.AddOn {
		var addOn bdm.AddOn
		err := json.Unmarshal([]byte(addon), &addOn)
		if err != nil {
			// This should never happen, because test data is valid
			panic("Loading yaml failed")
		}
		return &addOn
	}

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when deploying coredns", func() {
		BeforeEach(func() {
			boshdns.SetBoshDNSDockerImage("coredns/coredns:1.7.0")
			boshdns.SetClusterDomain("cluster.local")

			dns := boshdns.NewBoshDomainNameService(bdm.InstanceGroups{})

			err := dns.Add(loadAddOn(handlerAddon))
			Expect(err).NotTo(HaveOccurred())
			err = dns.Add(loadAddOn(aliasAddon))
			Expect(err).NotTo(HaveOccurred())

			cm, err := dns.CorefileConfigMap(env.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tearDown, err := env.CreateConfigMap(env.Namespace, cm)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			deployment := dns.Deployment(env.Namespace, "default")
			tearDown, err = env.CreateDeployment(env.Namespace, deployment)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
		})

		It("config is valid and deployment starts", func() {
			err := env.WaitForDeployment(env.Namespace, boshdns.AppName, 0)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
