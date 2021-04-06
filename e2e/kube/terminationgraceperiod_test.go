package kube_test

import (
	"path"
	"strconv"
	"time"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const graceTime = 70 // this has to match with quarks-gora-termination.yaml

var _ = Describe("Instance group", func() {
	When("specifying terminationGracePeriodSeconds in env.bosh.agent.settings", func() {
		BeforeEach(func() {
			By("Creating bdpl")
			f := path.Join(examplesDir, "bosh-deployment/quarks-gora-termination.yaml")
			err := cmdHelper.Create(namespace, f)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pods")
			err = kubectl.Wait(namespace, "ready", "pod/quarks-gora-0", kubectl.PollTimeout)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delay drain script termination meanwhile the instancegroup is running", func() {
			pod := "quarks-gora-0"

			uid, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.metadata.uid}")
			Expect(err).ToNot(HaveOccurred())

			// Make sure we set up sane defaults in the example and we don't change that accidentally.
			// graceTime has to be > 30, plus additional 10 to avoid test flakyness.
			// This test scenario is whenever we exceed Kubernetes default termination grace period which is 30s.
			Expect(graceTime > 40).To(BeTrue())

			requestedGrace, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.spec.terminationGracePeriodSeconds}")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(requestedGrace)).To(Equal(strconv.Itoa(graceTime)))

			go func() {
				_ = kubectl.Delete("pod", pod, "-n", namespace)
			}()

			isContainerUp := func() bool {
				status, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.status.containerStatuses[?(@.name==\"quarks-gora-quarks-gora\")].state.running.startedAt}")
				Expect(err).ToNot(HaveOccurred())
				return len(status) != 0
			}

			differentUID := func() bool {
				uidCurrent, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.metadata.uid}")
				Expect(err).ToNot(HaveOccurred())
				return (string(uid) == string(uidCurrent))
			}

			isUp := func() bool {
				e, _ := kubectl.Exists(namespace, "pod", pod)
				if !e {
					return false
				}

				return differentUID() && isContainerUp()
			}

			// Make sure the gora container is up and running and it exceeds default 30s from Kubernetes.
			// As we have setted up graceTime as terminationGrace and we expect the drain job to not terminate, the quarks-gora
			// container should be up meanwhile the drain is running. We leave 10 extra seconds to avoid test flakyness
			By("checking the pod stays up")
			Consistently(isUp, time.Duration(time.Duration(graceTime-10)*time.Second), time.Duration(1*time.Second)).Should(BeTrue(), "pod failed to be up")

			By("checking a new pod starts up")
			// Eventually, we should have a new pod as we exhaust the terminationGrace. Uses graceTime^2 as time window (not using pow for opt.)
			Eventually(differentUID, time.Duration(time.Duration(graceTime*graceTime)*time.Second), time.Duration(1*time.Second)).Should(BeFalse())

			err = kubectl.Wait(namespace, "ready", "pod/quarks-gora-0", kubectl.PollTimeout)
			Expect(err).ToNot(HaveOccurred())
			Expect(isContainerUp()).To(BeTrue())
		})
	})
})
