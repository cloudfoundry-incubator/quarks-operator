package boshdns_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
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

var _ = Describe("BOSH DNS", func() {
	Context("bosh-dns", func() {
		It("reconciles dns stuff", func() {
			dns, err := boshdns.NewBoshDomainNameService(loadAddOn(), nil)
			Expect(err).NotTo(HaveOccurred())
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(appsv1.AddToScheme(scheme)).To(Succeed())

			client := fake.NewFakeClientWithScheme(scheme)
			counter := 0
			err = dns.Reconcile(context.Background(), "default", client, func(object v1.Object) error {
				counter++
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(counter).To(Equal(3))
		})
	})

	Context("simple-dns", func() {
		It("shorten long service names", func() {
			dns := boshdns.NewSimpleDomainNameService()
			Expect(len(dns.HeadlessServiceName("scheduler-scheduler-scheduler-scheduler-scheduler-scheduler-scheduler-scheduler"))).
				To(Equal(63))
		})

		It("reconciles does nothing", func() {
			dns := boshdns.NewSimpleDomainNameService()
			client := fake.NewFakeClientWithScheme(runtime.NewScheme())
			err := dns.Reconcile(context.Background(), "default", client, func(object v1.Object) error {
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
