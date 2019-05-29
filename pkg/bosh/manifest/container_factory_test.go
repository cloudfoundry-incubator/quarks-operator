package manifest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	fakes "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
)

var _ = Describe("ContainerFactory", func() {
	var (
		cf         *ContainerFactory
		bpmConfigs bpm.Configs
		rip        *fakes.FakeReleaseImageProvider
	)

	BeforeEach(func() {
		rip = &fakes.FakeReleaseImageProvider{}
		rip.GetReleaseImageReturns("", nil)
		cf = NewContainerFactory("fake-manifest", "fake-ig", rip, bpmConfigs)
	})

	Context("JobsToInitContainers", func() {
		var jobs []Job

		BeforeEach(func() {
			jobs = []Job{
				Job{Name: "fake-job"},
				Job{Name: "other-job"},
			}
		})

		act := func() ([]corev1.Container, error) {
			return cf.JobsToInitContainers(jobs)
		}

		It("generates per job directories", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(3))
			Expect(containers[2].Name).To(Equal("create-dirs-fake-ig"))
			Expect(containers[2].Args).To(ContainElement("mkdir -p /var/vcap/data/fake-job /var/vcap/data/sys/log/fake-job /var/vcap/data/sys/run/fake-job /var/vcap/sys/log/fake-job /var/vcap/sys/run/fake-job /var/vcap/data/other-job /var/vcap/data/sys/log/other-job /var/vcap/data/sys/run/other-job /var/vcap/sys/log/other-job /var/vcap/sys/run/other-job"))
			Expect(containers[2].VolumeMounts).To(HaveLen(2))
		})
	})
})
