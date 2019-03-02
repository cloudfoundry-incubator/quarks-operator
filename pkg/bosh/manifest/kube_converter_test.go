package manifest_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"

	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

			kubeConfig, _ = m.ConvertToKube()
			Expect(kubeConfig.Variables[0].Name).To(Equal("abc-123.def-456"))
		})

		It("trims secret names to 63 characters", func() {
			m.Name = "foo"
			m.Variables[0].Name = "this-is-waaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay-too-long"

			kubeConfig, _ = m.ConvertToKube()
			Expect(kubeConfig.Variables[0].Name).To(Equal("foo.this-is-waaaaaaaaaaaaaaaaaaa0bef0f482cfb7313e03e6bd86b53ef3"))
		})

		It("converts password variables", func() {
			kubeConfig, _ = m.ConvertToKube()
			Expect(len(kubeConfig.Variables)).To(Equal(1))

			var1 := kubeConfig.Variables[0]
			Expect(var1.Name).To(Equal("foo-deployment.adminpass"))
			Expect(var1.Spec.Type).To(Equal(esv1.Password))
			Expect(var1.Spec.SecretName).To(Equal("foo-deployment.adminpass"))
		})

		It("converts rsa key variables", func() {
			m.Variables[0] = manifest.Variable{
				Name: "adminkey",
				Type: "rsa",
			}
			kubeConfig, _ = m.ConvertToKube()
			Expect(len(kubeConfig.Variables)).To(Equal(1))

			var1 := kubeConfig.Variables[0]
			Expect(var1.Name).To(Equal("foo-deployment.adminkey"))
			Expect(var1.Spec.Type).To(Equal(esv1.RSAKey))
			Expect(var1.Spec.SecretName).To(Equal("foo-deployment.adminkey"))
		})

		It("converts ssh key variables", func() {
			m.Variables[0] = manifest.Variable{
				Name: "adminkey",
				Type: "ssh",
			}
			kubeConfig, _ = m.ConvertToKube()
			Expect(len(kubeConfig.Variables)).To(Equal(1))

			var1 := kubeConfig.Variables[0]
			Expect(var1.Name).To(Equal("foo-deployment.adminkey"))
			Expect(var1.Spec.Type).To(Equal(esv1.SSHKey))
			Expect(var1.Spec.SecretName).To(Equal("foo-deployment.adminkey"))
		})

		It("converts certificate variables", func() {
			m.Variables[0] = manifest.Variable{
				Name: "foo-cert",
				Type: "certificate",
				Options: &manifest.VariableOptions{
					CommonName:       "example.com",
					AlternativeNames: []string{"foo.com", "bar.com"},
					IsCA:             true,
					CA:               "theca",
					ExtendedKeyUsage: []manifest.AuthType{manifest.ClientAuth},
				},
			}
			kubeConfig, _ = m.ConvertToKube()
			Expect(len(kubeConfig.Variables)).To(Equal(1))

			var1 := kubeConfig.Variables[0]
			Expect(var1.Name).To(Equal("foo-deployment.foo-cert"))
			Expect(var1.Spec.Type).To(Equal(esv1.Certificate))
			Expect(var1.Spec.SecretName).To(Equal("foo-deployment.foo-cert"))
			request := var1.Spec.Request.CertificateRequest
			Expect(request.CommonName).To(Equal("example.com"))
			Expect(request.AlternativeNames).To(Equal([]string{"foo.com", "bar.com"}))
			Expect(request.IsCA).To(Equal(true))
			Expect(request.CARef.Name).To(Equal("foo-deployment.theca"))
			Expect(request.CARef.Key).To(Equal("certificate"))
		})
	})

	Context("convert service lifecycle to instance groups", func() {
		It("when the lifecycle is set to service", func() {
			kubeConfig, err := m.ConvertToKube()
			Expect(err).ShouldNot(HaveOccurred())
			anExtendedSts := kubeConfig.ExtendedSts[0].Spec.Template.Spec.Template
			Expect(anExtendedSts.Name).To(Equal("diego-cell"))

			// Test containers in the extended statefulset
			Expect(anExtendedSts.Spec.Containers[0].Image).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3-release:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
			Expect(anExtendedSts.Spec.Containers[0].Command[0]).To(Equal("while true; do ping localhost;done"))
			Expect(anExtendedSts.Spec.Containers[0].Name).To(Equal("cflinuxfs3-rootfs-setup"))

			// Test init containers in the extended statefulset
			Expect(anExtendedSts.Spec.InitContainers[0].Image).To(Equal("/:0.0.1"))
			Expect(anExtendedSts.Spec.InitContainers[0].Name).To(Equal("diego-cell"))
			Expect(anExtendedSts.Spec.InitContainers[1].Image).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3-release:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
			Expect(anExtendedSts.Spec.InitContainers[1].Command[0]).To(Equal("echo \"\""))
			Expect(anExtendedSts.Spec.InitContainers[1].Name).To(Equal("cflinuxfs3-rootfs-setup"))
		})
	})

	Context("convert errand lifecycle to instance groups", func() {
		It("when the lifecycle is set to errand", func() {
			kubeConfig, err := m.ConvertToKube()
			Expect(err).ShouldNot(HaveOccurred())
			anExtendedJob := kubeConfig.ExtendedJob[0]

			Expect(len(kubeConfig.ExtendedJob)).To(Equal(1))
			Expect(len(kubeConfig.ExtendedJob)).ToNot(Equal(2))
			Expect(anExtendedJob.Name).To(Equal("redis-slave"))

			// Test containers in the extended job
			Expect(anExtendedJob.Spec.Template.Spec.Containers[0].Name).To(Equal("redis-server"))
			Expect(anExtendedJob.Spec.Template.Spec.Containers[0].Image).To(Equal("hub.docker.com/cfcontainerization/redis-release:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
			Expect(anExtendedJob.Spec.Template.Spec.Containers[0].Command[0]).To(Equal("while true; do ping localhost;done"))

			// Test init containers in the extended job
			Expect(anExtendedJob.Spec.Template.Spec.InitContainers[0].Image).To(Equal("/:0.0.1"))
			Expect(anExtendedJob.Spec.Template.Spec.InitContainers[0].Name).To(Equal("redis-slave"))
			Expect(anExtendedJob.Spec.Template.Spec.InitContainers[1].Image).To(Equal("hub.docker.com/cfcontainerization/redis-release:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
			Expect(anExtendedJob.Spec.Template.Spec.InitContainers[1].Command[0]).To(Equal("echo \"\""))

		})
	})

	Context("GetReleaseImage", func() {
		It("reports an error if the instance group was not found", func() {
			_, err := m.GetReleaseImage("unknown-instancegroup", "redis-server")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("reports an error if the stemcell was not found", func() {
			m.Stemcells = []*manifest.Stemcell{}
			_, err := m.GetReleaseImage("redis-slave", "redis-server")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("reports an error if the job was not found", func() {
			_, err := m.GetReleaseImage("redis-slave", "unknown-job")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("reports an error if the release was not found", func() {
			m.Releases = []*manifest.Release{}
			_, err := m.GetReleaseImage("redis-slave", "redis-server")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("calculates the release image name", func() {
			releaseImage, err := m.GetReleaseImage("redis-slave", "redis-server")
			Expect(err).ToNot(HaveOccurred())
			Expect(releaseImage).To(Equal("hub.docker.com/cfcontainerization/redis-release:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
		})

		It("uses the release stemcell information if it is set", func() {
			releaseImage, err := m.GetReleaseImage("diego-cell", "cflinuxfs3-rootfs-setup")
			Expect(err).ToNot(HaveOccurred())
			Expect(releaseImage).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3-release:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
		})
	})
})
