package controllers_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controllers", func() {
	Describe("AddToScheme", func() {
		It("registers our schemes with the operator", func() {
			scheme := scheme.Scheme
			controllers.AddToScheme(scheme)
			kinds := []string{}
			for k, _ := range scheme.AllKnownTypes() {
				kinds = append(kinds, k.Kind)
			}
			Expect(kinds).To(ContainElement("BOSHDeployment"))
			Expect(kinds).To(ContainElement("ExtendedStatefulSet"))
		})
	})

	// "AddToManager" tested via integration tests
})
