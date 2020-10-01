package boshdns_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
)

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
            "domain": "_.cell.service.cf.internal",
            "targets": [
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "diego-cell",
                "network": "default",
                "query": "_"
              },
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "windows2012R2-cell",
                "network": "default",
                "query": "_"
              },
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "windows2016-cell",
                "network": "default",
                "query": "_"
              },
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "windows1803-cell",
                "network": "default",
                "query": "_"
              },
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "windows2019-cell",
                "network": "default",
                "query": "_"
              },
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "isolated-diego-cell",
                "network": "default",
                "query": "_"
              }
            ]
          },
          {
            "domain": "auctioneer.service.cf.internal",
            "targets": [
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "scheduler",
                "network": "default",
                "query": "q-s4"
              }
            ]
          },
           {
            "domain": "bbs1.service.cf.internal",
            "targets": [
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "diego-api",
                "network": "default",
                "query": "q-s4"
              }
            ]
          },
         {
            "domain": "bbs.service.cf.internal",
            "targets": [
              {
                "deployment": "cf",
                "domain": "bosh",
                "instance_group": "diego-api",
                "network": "default",
                "query": "q-s4"
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
          },
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

var _ = Describe("BOSHDomainNameService", func() {
	Context("Apply", func() {
		var (
			client *cfakes.FakeClient
			dns    *boshdns.BoshDomainNameService
		)

		loadAddOn := func(addon string) *manifest.AddOn {
			var addOn manifest.AddOn
			err := json.Unmarshal([]byte(addon), &addOn)
			if err != nil {
				// This should never happen, because test data is valid
				panic("Loading yaml failed")
			}
			return &addOn
		}

		getCorefile := func(client *cfakes.FakeClient) string {
			_, obj, _ := client.CreateArgsForCall(0)
			cm, ok := obj.(*corev1.ConfigMap)
			Expect(ok).To(BeTrue())
			corefile, ok := cm.Data["Corefile"]
			Expect(ok).To(BeTrue())
			return corefile
		}

		When("BOSHDNS Addon has aliases", func() {
			BeforeEach(func() {
				igs := manifest.InstanceGroups{
					&manifest.InstanceGroup{Name: "scheduler", AZs: []string{"az1", "az2"}},
					&manifest.InstanceGroup{Name: "diego-api", AZs: []string{"az1", "az2"}},
					&manifest.InstanceGroup{Name: "bits", AZs: []string{"az1", "az2"}},
					&manifest.InstanceGroup{Name: "diego-cell", AZs: []string{"az1", "az2"}, Instances: 1},
				}
				dns = boshdns.NewBoshDomainNameService(igs)
				err := dns.Add(loadAddOn(aliasAddon))
				Expect(err).NotTo(HaveOccurred())

				client = &cfakes.FakeClient{}
				// Needed to get along with CreateOrUpdate
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
			})

			It("creates coredns resources and generates resources", func() {
				counter := 0
				err := dns.Apply(context.Background(), "default", client, func(object v1.Object) error {
					counter++
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(3)) // configmap, deployment, service

				By("calling setOwner", func() {
					Expect(counter).To(Equal(3))
				})

				By("checking for entries in corefile")
				corefile := getCorefile(client)
				Expect(corefile).To(ContainSubstring(`
	template IN A bits.service.cf.internal {
		match ^bits\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME bits.default.svc."
		upstream`))
				Expect(corefile).To(ContainSubstring(`
	template IN AAAA bits.service.cf.internal {
		match ^bits\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME bits.default.svc."
		upstream`))
				Expect(corefile).To(ContainSubstring(`
	template IN CNAME bbs1.service.cf.internal {
		match ^bbs1\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME diego-api.default.svc."
		upstream
	}`))

				By("checking for entries which are missing an instance group, but do not use _ query")
				Expect(corefile).To(ContainSubstring(`
	template IN A uaa.service.cf.internal {
		match ^uaa\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME uaa.default.svc."
		upstream`))

				By("checking for entries for diego-cells in mutli-zone")
				Expect(corefile).To(ContainSubstring(`
	template IN A diego-cell-z0-0.cell.service.cf.internal {
		match ^diego-cell-z0-0\.cell\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME diego-cell-z0-0.default.svc."
		upstream`))
			})
		})

		When("BOSHDNS Addon has handlers", func() {
			BeforeEach(func() {
				dns = boshdns.NewBoshDomainNameService(manifest.InstanceGroups{})
				err := dns.Add(loadAddOn(handlerAddon))
				Expect(err).NotTo(HaveOccurred())

				client = &cfakes.FakeClient{}
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
			})

			It("creates coredns corefile with handler statements", func() {
				err := dns.Apply(context.Background(), "default", client, func(object v1.Object) error { return nil })
				Expect(err).NotTo(HaveOccurred())

				By("checking for handler entries in corefile")
				corefile := getCorefile(client)
				Expect(corefile).To(ContainSubstring(`corp.intranet.local:8053 {`))
				Expect(corefile).To(ContainSubstring(`forward . dns://10.0.0.2 dns://127.0.0.1`))
				Expect(corefile).To(ContainSubstring(`forward . /etc/resolv.conf`))
			})
		})

		When("adding multiple dns addons", func() {
			BeforeEach(func() {
				dns = boshdns.NewBoshDomainNameService(manifest.InstanceGroups{})
				err := dns.Add(loadAddOn(handlerAddon))
				Expect(err).NotTo(HaveOccurred())
				err = dns.Add(loadAddOn(aliasAddon))
				Expect(err).NotTo(HaveOccurred())

				client = &cfakes.FakeClient{}
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
			})

			It("creates coredns corefile with handler statements", func() {
				err := dns.Apply(context.Background(), "default", client, func(object v1.Object) error { return nil })
				Expect(err).NotTo(HaveOccurred())

				By("checking for handler entries in corefile")
				corefile := getCorefile(client)
				Expect(corefile).To(ContainSubstring(`corp.intranet.local:8053 {`))
				Expect(corefile).To(ContainSubstring(`forward . dns://10.0.0.2 dns://127.0.0.1`))
				Expect(corefile).To(ContainSubstring(`forward . /etc/resolv.conf`))
				Expect(corefile).To(ContainSubstring(`
	template IN CNAME bbs1.service.cf.internal {
		match ^bbs1\.service\.cf\.internal\.$
		answer "{{ .Name }} 60 IN CNAME diego-api.default.svc."
		upstream
	}`))
			})
		})
	})
})
