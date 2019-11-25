package converter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("JobFactory", func() {
	var (
		factory *JobFactory
		m       *manifest.Manifest
		env     testing.Catalog
		err     error
	)

	BeforeEach(func() {
		m, err = env.DefaultBOSHManifest()
		Expect(err).NotTo(HaveOccurred())
		factory = NewJobFactory("namespace")
	})

	Describe("InstanceGroupManifestJob", func() {
		It("creates init containers", func() {
			qJob, err := factory.InstanceGroupManifestJob(*m)
			Expect(err).ToNot(HaveOccurred())
			jobIG := qJob.Spec.Template.Spec
			// Test init containers in the ig manifest qJob
			Expect(jobIG.Template.Spec.InitContainers[0].Name).To(Equal("spec-copier-redis"))
			Expect(jobIG.Template.Spec.InitContainers[1].Name).To(Equal("spec-copier-cflinuxfs3"))
			Expect(jobIG.Template.Spec.InitContainers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
			Expect(jobIG.Template.Spec.InitContainers[1].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
		})

		It("handles an error when getting release image", func() {
			m.Stemcells = nil
			_, err := factory.InstanceGroupManifestJob(*m)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generation of gathering job failed for manifest"))
		})

		It("does not generate the instance group containers when its instances is zero", func() {
			m.InstanceGroups[0].Instances = 0
			qJob, err := factory.InstanceGroupManifestJob(*m)
			Expect(err).ToNot(HaveOccurred())
			jobIG := qJob.Spec.Template.Spec
			Expect(len(jobIG.Template.Spec.InitContainers)).To(BeNumerically("<", 2))
			Expect(len(jobIG.Template.Spec.Containers)).To(BeNumerically("<", 2))
		})
	})

	Describe("BPMConfigsJob", func() {
		It("has one spec-copier init container per instance group", func() {
			job, err := factory.BPMConfigsJob(*m)
			Expect(err).ToNot(HaveOccurred())

			spec := job.Spec.Template.Spec.Template.Spec
			Expect(job.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))

			Expect(len(spec.InitContainers)).To(Equal(len(m.InstanceGroups)))
			Expect(spec.InitContainers[0].Name).To(ContainSubstring("spec-copier-"))
		})

		It("has one bpm-configs container per instance group", func() {
			job, err := factory.BPMConfigsJob(*m)
			Expect(err).ToNot(HaveOccurred())

			spec := job.Spec.Template.Spec.Template.Spec
			Expect(len(spec.Containers)).To(Equal(len(m.InstanceGroups)))
			Expect(spec.Containers[0].Name).To(Equal(m.InstanceGroups[0].Name))
			Expect(spec.Containers[0].Args).To(Equal([]string{"util", "bpm-configs"}))
		})

		It("does not generate the instance group containers when its instances is zero", func() {
			m.InstanceGroups[0].Instances = 0
			job, err := factory.BPMConfigsJob(*m)
			Expect(err).ToNot(HaveOccurred())

			spec := job.Spec.Template.Spec.Template.Spec
			Expect(len(spec.InitContainers)).To(BeNumerically("<", 2))
			Expect(len(spec.Containers)).To(BeNumerically("<", 2))
		})
	})

	Describe("VariableInterpolationJob", func() {
		It("mounts variable secrets in the variable interpolation container", func() {
			job, err := factory.VariableInterpolationJob(*m)
			Expect(err).ToNot(HaveOccurred())
			Expect(job.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))

			podSpec := job.Spec.Template.Spec.Template.Spec

			volumes := []string{}
			for _, v := range podSpec.Volumes {
				volumes = append(volumes, v.Name)
			}
			Expect(volumes).To(ConsistOf("with-ops", "var-adminpass"))

			mountPaths := []string{}
			for _, p := range podSpec.Containers[0].VolumeMounts {
				mountPaths = append(mountPaths, p.MountPath)
			}
			Expect(mountPaths).To(ConsistOf(
				"/var/run/secrets/deployment/",
				"/var/run/secrets/variables/adminpass",
			))
		})
	})
})
