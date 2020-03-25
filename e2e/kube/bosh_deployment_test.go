package kube_test

import (
	"path"
	"time"

	"code.cloudfoundry.org/quarks-utils/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BOSHDeployment", func() {
	Context("when restarting operator", func() {
		kubectl := testing.NewKubectl()

		BeforeEach(func() {
			By("Creating bdpl")
			f := path.Join(examplesDir, "bosh-deployment/boshdeployment-with-custom-variable.yaml")
			err := testing.Create(namespace, f)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pods")
			err = kubectl.Wait(namespace, "ready", "pod/nats-0", kubectl.PollTimeout)
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.Wait(namespace, "ready", "pod/nats-1", kubectl.PollTimeout)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not create unexpected resources", func() {
			status, err := kubectl.PodStatus(namespace, "nats-0")
			Expect(err).ToNot(HaveOccurred(), "error getting pod start time")
			startTime := status.StartTime
			err = testing.RestartOperator(operatorNamespace)
			Expect(err).ToNot(HaveOccurred(), "error restarting cf-operator")

			By("Checking for pod not restarted")
			time.Sleep(10 * time.Second)
			status, err = kubectl.PodStatus(namespace, "nats-0")
			Expect(err).ToNot(HaveOccurred(), "error getting pod start time")
			Expect(status.StartTime).To(Equal(startTime), "error pod must not be restarted")

			By("Checking for secrets not created")
			exist, err := kubectl.SecretExists(namespace, "bpm.nats-v2")
			Expect(err).ToNot(HaveOccurred(), "error getting secret/bpm.nats-v2")
			Expect(exist).To(BeFalse(), "error unexpected bpm info secret is created")

			exist, err = kubectl.SecretExists(namespace, "desired-manifest-v2")
			Expect(err).ToNot(HaveOccurred(), "error getting secret/desired-manifest-v2")
			Expect(exist).To(BeFalse(), "error unexpected desire manifest is created")

			exist, err = kubectl.SecretExists(namespace, "ig-resolved.nats-v2")
			Expect(err).ToNot(HaveOccurred(), "error getting secret/ig-resolved.nats-v2")
			Expect(exist).To(BeFalse(), "error unexpected properties secret is created")
		})
	})
})
