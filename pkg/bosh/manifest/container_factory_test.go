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
		jobs       []Job
	)

	BeforeEach(func() {
		rip = &fakes.FakeReleaseImageProvider{}
		rip.GetReleaseImageReturns("", nil)

		jobs = []Job{
			Job{Name: "fake-job"},
			Job{Name: "other-job"},
		}
	})

	JustBeforeEach(func() {
		cf = NewContainerFactory("fake-manifest", "fake-ig", rip, bpmConfigs)
	})

	Context("JobsToContainers", func() {
		BeforeEach(func() {
			bpmConfigs = bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{Name: "fake-process"},
					},
				},
				"other-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{
							Name:         "fake-process",
							Capabilities: []string{"CHOWN", "AUDIT_CONTROL"},
						},
					},
				},
			}
		})

		act := func() ([]corev1.Container, error) {
			return cf.JobsToContainers(jobs)
		}

		It("adds the sys volume", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(2))
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "sys-dir",
					ReadOnly:         false,
					MountPath:        "/var/vcap/sys",
					SubPath:          "",
					MountPropagation: nil,
				}))

		})

		It("adds linux capabilities to containers", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(containers[1].SecurityContext.Capabilities.Add[0])).To(Equal("CHOWN"))
			Expect(string(containers[1].SecurityContext.Capabilities.Add[1])).To(Equal("AUDIT_CONTROL"))
		})
	})

	Context("JobsToInitContainers", func() {
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
