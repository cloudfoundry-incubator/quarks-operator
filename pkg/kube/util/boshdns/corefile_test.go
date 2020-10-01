package boshdns_test

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
)

var _ = Describe("Corefile", func() {
	Context("Create", func() {

		var (
			corefile *boshdns.Corefile
			igs      manifest.InstanceGroups
		)

		const (
			aliasAddon = `
{
  "aliases": [
    {
      "domain": "_.cell.service.cf.internal",
      "targets": [
        {
          "deployment": "cf",
          "domain": "bosh",
          "instance_group": "diego-cell",
          "network": "default",
          "query": "_"
        }
      ]
    },
    {
      "domain": "bits.service.cf.internal",
      "targets": [
        {
          "deployment": "cf",
          "domain": "bosh",
          "instance_group": "bits",
          "network": "default",
          "query": "*"
        }
      ]
    }
  ]
}
`

			handlerAddon = `
{
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
`
		)

		load := func(addonProps string) map[string]interface{} {
			var props map[string]interface{}
			err := json.Unmarshal([]byte(addonProps), &props)
			if err != nil {
				// This should never happen, because test data is valid
				panic("Loading yaml failed")
			}
			return props
		}

		BeforeEach(func() {
			corefile = &boshdns.Corefile{}
			igs = manifest.InstanceGroups{
				&manifest.InstanceGroup{Name: "scheduler", AZs: []string{"az1", "az2"}},
			}
		})

		When("combining two addons", func() {
			It("forwards to a tls dns server", func() {
				err := corefile.Add(load(aliasAddon))
				Expect(err).NotTo(HaveOccurred())
				err = corefile.Add(load(handlerAddon))
				Expect(err).NotTo(HaveOccurred())

				corefile, err := corefile.Create("default", igs)
				Expect(err).NotTo(HaveOccurred())

				Expect(corefile).To(ContainSubstring(`corp.intranet.local:8053 {`))
				Expect(corefile).To(ContainSubstring(`forward . dns://10.0.0.2 dns://127.0.0.1`))
				Expect(corefile).To(ContainSubstring(`forward . /etc/resolv.conf`))
				Expect(corefile).To(ContainSubstring(`
	template IN A bits.service.cf.internal {
		match ^bits\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME bits.default.svc."
		upstream`))
			})
		})

		When("setting DNS server type", func() {
			It("translates to a valid coredns protocol", func() {
				tests := []struct {
					Type     string
					Protocol string
				}{
					{"dns", "dns"},
					{"tls", "tls"},
					{"http", "https"},
					{"https", "https"},
					{"grpc", "grpc"},
				}

				for _, t := range tests {
					err := corefile.Add(load(strings.Replace(handlerAddon, "dns", t.Type, 1)))
					Expect(err).NotTo(HaveOccurred())

					corefile, err := corefile.Create("default", igs)
					Expect(err).NotTo(HaveOccurred())

					Expect(corefile).To(ContainSubstring(fmt.Sprintf(`forward . %[1]s://10.0.0.2 %[1]s://127.0.0.1`, t.Protocol)))
				}
			})
		})
	})
})
