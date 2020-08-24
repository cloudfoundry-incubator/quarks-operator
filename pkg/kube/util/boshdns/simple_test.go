package boshdns_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
)

var _ = Describe("SimpleDomainNameService", func() {
	Context("Apply", func() {
		It("does nothing", func() {
			dns := boshdns.NewSimpleDomainNameService()
			client := fake.NewFakeClientWithScheme(runtime.NewScheme())
			err := dns.Apply(context.Background(), "default", client, func(object v1.Object) error {
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
