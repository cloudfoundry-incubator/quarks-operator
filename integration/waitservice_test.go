package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("WaitService PodMutator", func() {
	const (
		firstServiceName  = "nats-headless"
		secondServiceName = "dummy"
		waitForKey        = "wait-for-%s"
	)

	var (
		tearDowns                      []machine.TearDownFunc
		pod                            corev1.Pod
		service                        corev1.Service
		firstServiceWaitContainerName  = fmt.Sprintf(waitForKey, firstServiceName)
		secondServiceWaitContainerName = fmt.Sprintf(waitForKey, secondServiceName)
	)

	containerNames := func(initcontainer []corev1.Container) []string {
		names := make([]string, len(initcontainer))
		for i, c := range initcontainer {
			names[i] = c.Name
		}
		return names
	}

	waitingContainersExists := func(initcontainers []corev1.Container) {
		Expect(containerNames(initcontainers)).To(ContainElement(firstServiceWaitContainerName))
		Expect(containerNames(initcontainers)).To(ContainElement(secondServiceWaitContainerName))
	}

	act := func(pod corev1.Pod) {
		tearDown, err := env.CreatePod(env.Namespace, pod)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)
		err = env.WaitForPod(env.Namespace, pod.GetName())
		Expect(err).NotTo(HaveOccurred())
	}

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when service is available and pod has wait-for annotation", func() {
		BeforeEach(func() {
			service = env.DummyService(firstServiceName)
			tearDown, err := env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
			pod = env.WaitingPod("waiting", firstServiceName)
		})

		It("injects an initcontainer in the pod", func() {
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(1))
				Expect(containerNames(p.Spec.InitContainers)).To(ContainElement(firstServiceWaitContainerName))
			})
		})

		It("adds an initcontainer in a pod which already has initcontainers", func() {
			pod.Spec.InitContainers = []corev1.Container{env.EchoContainer("test")}
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(2))
				Expect(containerNames(p.Spec.InitContainers)).To(ContainElement(firstServiceWaitContainerName))
				Expect(containerNames(p.Spec.InitContainers)).To(ContainElement("test"))
			})
		})
	})

	Context("when both services are available and pod has wait-for annotation", func() {
		BeforeEach(func() {
			service = env.DummyService(firstServiceName)
			tearDown, err := env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			service = env.DummyService(secondServiceName)
			tearDown, err = env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
			pod = env.WaitingPod("waiting", firstServiceName, secondServiceName)
		})

		It("injects an initcontainer in the pod", func() {
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(2))
				waitingContainersExists(p.Spec.InitContainers)
			})
		})

		It("adds an initcontainer in a pod which already has initcontainers", func() {
			pod.Spec.InitContainers = []corev1.Container{env.EchoContainer("test")}
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(3))
				waitingContainersExists(p.Spec.InitContainers)
				Expect(containerNames(p.Spec.InitContainers)).To(ContainElement("test"))
			})
		})
	})

	Context("when pod has no wait-for annotation", func() {
		BeforeEach(func() {
			pod = env.DefaultPod("not-waiting")
		})

		It("does not add an initcontainer in the pod", func() {
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(0))
			})
		})
	})

	Context("when service is not available yet and pod has wait-for annotation", func() {
		BeforeEach(func() {
			pod = env.WaitingPod("waiting", firstServiceName, secondServiceName)
			tearDown, err := env.CreatePod(env.Namespace, pod)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
		})

		It("init container waits until the services are available", func() {
			p, err := env.GetPod(env.Namespace, pod.GetName())
			Expect(err).NotTo(HaveOccurred())

			// First init container will wait for the service
			err = env.WaitForInitContainerRunning(env.Namespace, pod.GetName(), firstServiceWaitContainerName)
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodContainerLogMsg(env.Namespace, pod.GetName(), firstServiceWaitContainerName, fmt.Sprintf("Waiting for %s to be reachable", firstServiceName))
			Expect(err).NotTo(HaveOccurred())

			service = env.DummyService(firstServiceName)
			tearDown, err := env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			// After the service is created, it starts waiting for the other one
			err = env.WaitForInitContainerRunning(env.Namespace, pod.GetName(), secondServiceWaitContainerName)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForPodContainerLogMsg(env.Namespace, pod.GetName(), secondServiceWaitContainerName, fmt.Sprintf("Waiting for %s to be reachable", secondServiceName))
			Expect(err).NotTo(HaveOccurred())

			service = env.DummyService(secondServiceName)
			tearDown, err = env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			Expect(p.Spec.InitContainers).To(HaveLen(2))
			waitingContainersExists(p.Spec.InitContainers)

			By("checking the initcontainers and the pod state", func() {

				// Match that we have actually waited for both services
				for _, container := range []string{firstServiceWaitContainerName, secondServiceWaitContainerName} {
					err = env.WaitForPodContainerLogMatchRegexp(env.Namespace, pod.GetName(), container, "real.*m.*s")
					Expect(err).NotTo(HaveOccurred())
					err = env.WaitForPodContainerLogMatchRegexp(env.Namespace, pod.GetName(), container, "user.*m.*s")
					Expect(err).NotTo(HaveOccurred())
					err = env.WaitForPodContainerLogMatchRegexp(env.Namespace, pod.GetName(), container, "sys.*m.*s")
					Expect(err).NotTo(HaveOccurred())
				}

				err = env.WaitForPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
