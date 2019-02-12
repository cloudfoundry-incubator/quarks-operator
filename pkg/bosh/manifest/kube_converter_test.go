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

	JustBeforeEach(func() {
		kubeConfig = m.ConvertToKube()
	})

	Context("converting variables", func() {
		It("converts password variables", func() {
			Expect(len(kubeConfig.Variables)).To(Equal(1))

			var1 := kubeConfig.Variables[0]
			Expect(var1.Name).To(Equal("foo-deployment.adminpass"))
			Expect(var1.Spec.Type).To(Equal(esv1.Password))
			Expect(var1.Spec.SecretName).To(Equal("foo-deployment.adminpass"))
		})
	})
})
