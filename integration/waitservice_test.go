package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("WaitService PodMutator", func() {
	const (
		deploymentName = "nats-deployment"
	)

	var (
		tearDowns []machine.TearDownFunc
		pod       corev1.Pod
		service   corev1.Service
	)

	containerNames := func(initcontainer []corev1.Container) []string {
		names := make([]string, len(initcontainer))
		for i, c := range initcontainer {
			names[i] = c.Name
		}
		return names
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
			service = env.NatsService(deploymentName)
			tearDown, err := env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
			pod = env.WaitingPod("waiting", "nats-headless")
		})

		It("injects an initcontainer in the pod", func() {
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(1))
				Expect(containerNames(p.Spec.InitContainers)).To(ContainElement("wait-for"))
			})
		})

		It("adds an initcontainer in a pod which already has initcontainers", func() {
			pod.Spec.InitContainers = []corev1.Container{env.EchoContainer("test")}
			act(pod)

			By("checking the created initcontainer", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.InitContainers).To(HaveLen(2))
				Expect(containerNames(p.Spec.InitContainers)).To(ContainElement("wait-for"))
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
			pod = env.WaitingPod("waiting", "nats-headless")
			tearDown, err := env.CreatePod(env.Namespace, pod)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
		})

		It("init container waits until the service is available", func() {
			p, err := env.GetPod(env.Namespace, pod.GetName())
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForInitContainerRunning(env.Namespace, pod.GetName(), "wait-for")
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodContainerLogMsg(env.Namespace, pod.GetName(), "wait-for", "Waiting for nats-headless to be reachable")
			Expect(err).NotTo(HaveOccurred())

			service = env.NatsService(deploymentName)
			tearDown, err := env.CreateService(env.Namespace, service)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			Expect(p.Spec.InitContainers).To(HaveLen(1))
			Expect(containerNames(p.Spec.InitContainers)).To(ContainElement("wait-for"))

			By("checking the initcontainer and the pod state", func() {

				// Match that we have actually waited
				err = env.WaitForPodContainerLogMatchRegexp(env.Namespace, pod.GetName(), "wait-for", "real.*m.*s")
				Expect(err).NotTo(HaveOccurred())
				err = env.WaitForPodContainerLogMatchRegexp(env.Namespace, pod.GetName(), "wait-for", "user.*m.*s")
				Expect(err).NotTo(HaveOccurred())
				err = env.WaitForPodContainerLogMatchRegexp(env.Namespace, pod.GetName(), "wait-for", "sys.*m.*s")
				Expect(err).NotTo(HaveOccurred())

				err = env.WaitForPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
