package manifest_test

import (
	"context"
	"encoding/json"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const boshDNSAddOn = `
{
  "jobs": [
    {
      "name": "bosh-dns-aliases",
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
      },
      "release": "bosh-dns-aliases"
    }
  ],
  "name": "bosh-dns-aliases"
}
`

func loadAddOn() *manifest.AddOn {
	var addOn manifest.AddOn
	err := json.Unmarshal([]byte(boshDNSAddOn), &addOn)
	if err != nil {
		// This should never happen, because boshDNSAddOn is a constant
		panic("Loading yaml failed")
	}
	return &addOn
}

var _ = Describe("kube converter", func() {

	Context("bosh-dns", func() {

		It("loads dns from addons correct", func() {
			dns, err := manifest.NewBoshDomainNameService("default", loadAddOn())
			Expect(err).NotTo(HaveOccurred())
			services := dns.FindServiceNames("scheduler", "")
			Expect(services).To(HaveLen(1))
			Expect(services[0]).To(Equal("auctioneer"))
			Expect(dns.HeadlessServiceName("scheduler", "")).To(Equal("auctioneer"))
		})
		It("returns the correct service names", func() {
			dns, err := manifest.NewBoshDomainNameService("default", loadAddOn())
			Expect(err).NotTo(HaveOccurred())
			diegoAPI := dns.FindServiceNames("diego-api", "cf")
			Expect(diegoAPI).To(ConsistOf("bbs", "bbs1"))
			uaa := dns.FindServiceNames("uaa", "cf")
			Expect(uaa).To(ConsistOf("uaa"))
		})
		It("returns the default name if there is no alias configured", func() {
			dns, err := manifest.NewBoshDomainNameService("default", loadAddOn())
			Expect(err).NotTo(HaveOccurred())
			invalid := dns.FindServiceNames("invalid", "cf")
			Expect(invalid).To(ConsistOf("cf-invalid"))

		})
		It("reads nameserver from resolve.conv", func() {
			ip := manifest.GetNameserverFromResolveConfig([]byte("search wdf.sap.corp global.corp.sap\nnameserver 10.17.122.10\n"))
			Expect(ip).To(Equal("10.17.122.10"))
		})
		It("reads default nameserver from resolve.conv", func() {
			ip := manifest.GetNameserverFromResolveConfig([]byte(""))
			Expect(ip).To(Equal("1.1.1.1"))
		})
		It("reconciles dns stuff", func() {
			d, err := manifest.NewBoshDomainNameService("default", loadAddOn())
			Expect(err).NotTo(HaveOccurred())
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(appsv1.AddToScheme(scheme)).To(Succeed())

			client := fake.NewFakeClientWithScheme(scheme)
			counter := 0
			err = d.Reconcile(context.TODO(), client, func(object v1.Object) error {
				counter++
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(counter).To(Equal(3))
		})
	})
	Context("simple-dns", func() {

		It("returns correct service names", func() {
			dns := manifest.NewSimpleDomainNameService()
			services := dns.FindServiceNames("scheduler", "sfc")
			Expect(services).To(HaveLen(1))
			Expect(services[0]).To(Equal("sfc-scheduler"))
			Expect(dns.HeadlessServiceName("scheduler", "sfc")).To(Equal("sfc-scheduler"))
		})
		It("shorten long service names", func() {
			dns := manifest.NewSimpleDomainNameService()
			Expect(len(dns.HeadlessServiceName(
				"scheduler-scheduler-scheduler-scheduler-scheduler-scheduler-scheduler-scheduler",
				"sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-sfc-"))).
				To(Equal(63))
		})
		It("reconciles does nothing", func() {
			dns := manifest.NewSimpleDomainNameService()

			client := fake.NewFakeClientWithScheme(runtime.NewScheme())

			err := dns.Reconcile(context.TODO(), client, func(object v1.Object) error {
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

})
