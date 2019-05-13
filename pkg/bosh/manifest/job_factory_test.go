package manifest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("JobFactory", func() {
	var (
		factory *manifest.JobFactory
		m       manifest.Manifest
		env     testing.Catalog
	)

	BeforeEach(func() {
		m = env.DefaultBOSHManifest()
		factory = manifest.NewJobFactory(m, "namespace")
	})

	Describe("DataGatheringJob", func() {
		It("creates init containers", func() {
			job, err := factory.DataGatheringJob()
			Expect(err).ToNot(HaveOccurred())
			jobDG := job.Spec.Template.Spec
			// Test init containers in the datagathering job
			Expect(jobDG.InitContainers[0].Name).To(Equal("spec-copier-redis"))
			Expect(jobDG.InitContainers[1].Name).To(Equal("spec-copier-cflinuxfs3"))
			Expect(jobDG.InitContainers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
			Expect(jobDG.InitContainers[1].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
		})
	})

	Describe("VariableInterpolationJob", func() {
		It("mounts variable secrets in the variable interpolation container", func() {
			job, err := factory.VariableInterpolationJob()
			Expect(err).ToNot(HaveOccurred())
			podSpec := job.Spec.Template.Spec

			volumes := []string{}
			for _, v := range podSpec.Volumes {
				volumes = append(volumes, v.Name)
			}
			Expect(volumes).To(ConsistOf("with-ops", "var-adminpass"))

			mountPaths := []string{}
			for _, p := range podSpec.Containers[0].VolumeMounts {
				mountPaths = append(mountPaths, p.MountPath)
			}
			Expect(mountPaths).To(ConsistOf("/var/run/secrets/deployment/", "/var/run/secrets/variables/adminpass"))
		})
	})
})
