package manifest_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"

	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var _ = Describe("kube converter", func() {
	var (
		m          manifest.Manifest
		kubeConfig manifest.KubeConfig
		env        testing.Catalog
	)

	BeforeEach(func() {
		m = env.DefaultBOSHManifest()
		format.TruncatedDiff = false
	})

	var _ = Describe("ConvertToKube", func() {
		Context("converting variables", func() {
			It("sanitizes secret names", func() {
				m.Name = "-abc_123.?!\"§$&/()=?"
				m.Variables[0].Name = "def-456.?!\"§$&/()=?-"

				kubeConfig, _ = m.ConvertToKube("foo")
				Expect(kubeConfig.Variables[0].Name).To(Equal("abc-123.var-def-456"))
			})

			It("trims secret names to 63 characters", func() {
				m.Name = "foo"
				m.Variables[0].Name = "this-is-waaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay-too-long"

				kubeConfig, _ = m.ConvertToKube("foo")
				Expect(kubeConfig.Variables[0].Name).To(Equal("foo.var-this-is-waaaaaaaaaaaaaa5bffdb0302ac051d11f52d2606254a5f"))
			})

			It("converts password variables", func() {
				kubeConfig, _ = m.ConvertToKube("foo")
				Expect(len(kubeConfig.Variables)).To(Equal(1))

				var1 := kubeConfig.Variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-adminpass"))
				Expect(var1.Spec.Type).To(Equal(esv1.Password))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-adminpass"))
			})

			It("converts rsa key variables", func() {
				m.Variables[0] = manifest.Variable{
					Name: "adminkey",
					Type: "rsa",
				}
				kubeConfig, _ = m.ConvertToKube("foo")
				Expect(len(kubeConfig.Variables)).To(Equal(1))

				var1 := kubeConfig.Variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-adminkey"))
				Expect(var1.Spec.Type).To(Equal(esv1.RSAKey))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-adminkey"))
			})

			It("converts ssh key variables", func() {
				m.Variables[0] = manifest.Variable{
					Name: "adminkey",
					Type: "ssh",
				}
				kubeConfig, _ = m.ConvertToKube("foo")
				Expect(len(kubeConfig.Variables)).To(Equal(1))

				var1 := kubeConfig.Variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-adminkey"))
				Expect(var1.Spec.Type).To(Equal(esv1.SSHKey))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-adminkey"))
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
				kubeConfig, _ = m.ConvertToKube("foo")
				Expect(len(kubeConfig.Variables)).To(Equal(1))

				var1 := kubeConfig.Variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-foo-cert"))
				Expect(var1.Spec.Type).To(Equal(esv1.Certificate))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-foo-cert"))
				request := var1.Spec.Request.CertificateRequest
				Expect(request.CommonName).To(Equal("example.com"))
				Expect(request.AlternativeNames).To(Equal([]string{"foo.com", "bar.com"}))
				Expect(request.IsCA).To(Equal(true))
				Expect(request.CARef.Name).To(Equal("foo-deployment.var-theca"))
				Expect(request.CARef.Key).To(Equal("certificate"))
			})

			It("mounts variable secrets in the variable interpolation container", func() {
				kubeConfig, err := m.ConvertToKube("foo")
				Expect(err).ToNot(HaveOccurred())
				job := kubeConfig.VariableInterpolationJob
				podSpec := job.Spec.Template.Spec

				volumes := []string{}
				for _, v := range podSpec.Volumes {
					volumes = append(volumes, v.Name)
				}
				Expect(volumes).To(ConsistOf("with-ops", "var-adminpass", "var-app-domain", "var-system-domain"))

				mountPaths := []string{}
				for _, p := range podSpec.Containers[0].VolumeMounts {
					mountPaths = append(mountPaths, p.MountPath)
				}
				Expect(mountPaths).To(ConsistOf("/var/run/secrets/deployment/", "/var/run/secrets/variables/adminpass",
					"/var/run/secrets/variables/app_domain", "/var/run/secrets/variables/system_domain"))
			})
		})

		Context("when invoking the data gathering job", func() {
			It("verify job init containers fields", func() {
				kubeConfig, err := m.ConvertToKube("foo")
				Expect(err).ShouldNot(HaveOccurred())
				jobDG := kubeConfig.DataGatheringJob.Spec.Template.Spec
				// Test init containers in the datagathering job
				Expect(jobDG.InitContainers[0].Name).To(Equal("spec-copier-redis"))
				Expect(jobDG.InitContainers[1].Name).To(Equal("spec-copier-cflinuxfs3"))
				Expect(jobDG.InitContainers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(jobDG.InitContainers[1].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
			})
		})

		Context("when the lifecycle is set to service", func() {
			It("converts the instance group to an ExtendedStatefulset", func() {
				kubeConfig, err := m.ConvertToKube("foo")
				Expect(err).ShouldNot(HaveOccurred())
				anExtendedSts := kubeConfig.InstanceGroups[0].Spec.Template.Spec.Template
				Expect(anExtendedSts.Name).To(Equal("diego-cell"))

				specCopierInitContainer := anExtendedSts.Spec.InitContainers[0]
				rendererInitContainer := anExtendedSts.Spec.InitContainers[1]

				// Test containers in the extended statefulset
				Expect(anExtendedSts.Spec.Containers[0].Image).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
				Expect(anExtendedSts.Spec.Containers[0].Command).To(BeNil())
				Expect(anExtendedSts.Spec.Containers[0].Name).To(Equal("cflinuxfs3-rootfs-setup"))

				// Test init containers in the extended statefulset
				Expect(specCopierInitContainer.Name).To(Equal("spec-copier-cflinuxfs3"))
				Expect(specCopierInitContainer.Image).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
				Expect(specCopierInitContainer.Command[0]).To(Equal("bash"))
				Expect(specCopierInitContainer.Name).To(Equal("spec-copier-cflinuxfs3"))
				Expect(rendererInitContainer.Image).To(Equal("/:"))
				Expect(rendererInitContainer.Name).To(Equal("renderer-diego-cell"))

				// Test shared volume setup
				Expect(anExtendedSts.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(anExtendedSts.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(specCopierInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(specCopierInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(rendererInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(rendererInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))

				// Test the renderer container setup
				Expect(rendererInitContainer.Env[0].Name).To(Equal("INSTANCE_GROUP_NAME"))
				Expect(rendererInitContainer.Env[0].Value).To(Equal("diego-cell"))
				Expect(rendererInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(rendererInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(rendererInitContainer.VolumeMounts[1].Name).To(Equal("jobs-dir"))
				Expect(rendererInitContainer.VolumeMounts[1].MountPath).To(Equal("/var/vcap/jobs"))
				Expect(rendererInitContainer.VolumeMounts[2].Name).To(Equal("ig-resolved"))
				Expect(rendererInitContainer.VolumeMounts[2].MountPath).To(Equal("/var/run/secrets/resolved-properties/diego-cell"))
			})
		})

		Context("when the lifecycle is set to errand", func() {
			It("converts the instance group to an ExtendedJob", func() {
				kubeConfig, err := m.ConvertToKube("foo")
				Expect(err).ShouldNot(HaveOccurred())
				anExtendedJob := kubeConfig.Errands[0]

				Expect(len(kubeConfig.Errands)).To(Equal(1))
				Expect(len(kubeConfig.Errands)).ToNot(Equal(2))
				Expect(anExtendedJob.Name).To(Equal("foo-deployment-redis-slave"))

				specCopierInitContainer := anExtendedJob.Spec.Template.Spec.InitContainers[0]
				rendererInitContainer := anExtendedJob.Spec.Template.Spec.InitContainers[1]

				// Test containers in the extended job
				Expect(anExtendedJob.Spec.Template.Spec.Containers[0].Name).To(Equal("redis-server"))
				Expect(anExtendedJob.Spec.Template.Spec.Containers[0].Image).To(Equal("hub.docker.com/cfcontainerization/redis:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
				Expect(anExtendedJob.Spec.Template.Spec.Containers[0].Command).To(BeNil())

				// Test init containers in the extended job
				Expect(specCopierInitContainer.Name).To(Equal("spec-copier-redis"))
				Expect(specCopierInitContainer.Image).To(Equal("hub.docker.com/cfcontainerization/redis:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
				Expect(specCopierInitContainer.Command[0]).To(Equal("bash"))
				Expect(rendererInitContainer.Image).To(Equal("/:"))
				Expect(rendererInitContainer.Name).To(Equal("renderer-redis-slave"))

				// Test shared volume setup
				Expect(anExtendedJob.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(anExtendedJob.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(specCopierInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(specCopierInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(rendererInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
				Expect(rendererInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))

				// Test mounting the resolved instance group properties in the renderer container
				Expect(rendererInitContainer.Env[0].Name).To(Equal("INSTANCE_GROUP_NAME"))
				Expect(rendererInitContainer.Env[0].Value).To(Equal("redis-slave"))
				Expect(rendererInitContainer.VolumeMounts[1].Name).To(Equal("jobs-dir"))
				Expect(rendererInitContainer.VolumeMounts[1].MountPath).To(Equal("/var/vcap/jobs"))
			})
		})
	})

	Describe("GetReleaseImage", func() {
		It("reports an error if the instance group was not found", func() {
			_, err := m.GetReleaseImage("unknown-instancegroup", "redis-server")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("reports an error if the stemcell was not found", func() {
			m.Stemcells = []*manifest.Stemcell{}
			_, err := m.GetReleaseImage("redis-slave", "redis-server")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stemcell could not be resolved"))
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
			Expect(releaseImage).To(Equal("hub.docker.com/cfcontainerization/redis:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
		})

		It("uses the release stemcell information if it is set", func() {
			releaseImage, err := m.GetReleaseImage("diego-cell", "cflinuxfs3-rootfs-setup")
			Expect(err).ToNot(HaveOccurred())
			Expect(releaseImage).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
		})

		var _ = Describe("AllVariableNames", func() {
			It("returns all variable names", func() {
				names, err := m.AllVariableNames()
				Expect(err).ToNot(HaveOccurred())
				Expect(names).To(ConsistOf([]string{"app_domain", "adminpass", "system_domain"}))
			})
		})
	})
})
