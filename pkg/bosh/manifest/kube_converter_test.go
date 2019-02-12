package manifest_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
)

var _ = Describe("ConvertToKube", func() {
	var (
		m          manifest.Manifest
		kubeConfig manifest.KubeConfig
		env        testing.Catalog
	)

	BeforeEach(func() {
		m = env.DefaultBOSHManifest()
	})

	Context("converting variables", func() {
		It("sanitizes secret names", func() {
			m.Name = "-abc_123.?!\"§$&/()=?"
			m.Variables[0].Name = "def-456.?!\"§$&/()=?-"

			kubeConfig = m.ConvertToKube()
			Expect(kubeConfig.Variables[0].Name).To(Equal("abc-123.def-456"))
		})

		It("trims secret names to 63 characters", func() {
			m.Name = "foo"
			m.Variables[0].Name = "this-is-waaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay-too-long"

			kubeConfig = m.ConvertToKube()
			Expect(kubeConfig.Variables[0].Name).To(Equal("foo.this-is-waaaaaaaaaaaaaaaaaaa0bef0f482cfb7313e03e6bd86b53ef3"))
		})

		It("converts password variables", func() {
			kubeConfig = m.ConvertToKube()
			Expect(len(kubeConfig.Variables)).To(Equal(1))

			var1 := kubeConfig.Variables[0]
			Expect(var1.Name).To(Equal("foo-deployment.adminpass"))
			Expect(var1.Spec.Type).To(Equal(esv1.Password))
			Expect(var1.Spec.SecretName).To(Equal("foo-deployment.adminpass"))
		})
	})
})
