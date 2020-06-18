package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("Entangled Pods PodMutator", func() {
	const (
		deploymentName = "nats-deployment"
		consumesNats   = `[{"name":"nats","type":"nats"}]`
		consumesNuts   = `[{"name":"nats","type":"nuts"}]`
	)

	var (
		tearDowns []machine.TearDownFunc
		pod       corev1.Pod
	)

	volumeNames := func(volumes []corev1.Volume) []string {
		names := make([]string, len(volumes))
		for i, v := range volumes {
			names[i] = v.Name
		}
		return names
	}

	volumeMountNames := func(mounts []corev1.VolumeMount) []string {
		names := make([]string, len(mounts))
		for i, m := range mounts {
			names[i] = m.Name
		}
		return names
	}

	updateEntanglementAnnotations := func(consumes string) {
		p, err := env.GetPod(env.Namespace, pod.GetName())
		Expect(err).NotTo(HaveOccurred())
		p.SetAnnotations(map[string]string{
			"quarks.cloudfoundry.org/deployment": deploymentName,
			"quarks.cloudfoundry.org/consumes":   consumes,
		})
		_, _, err = env.UpdatePod(env.Namespace, *p)
		Expect(err).NotTo(HaveOccurred())
	}

	act := func(pod corev1.Pod) {
		tearDown, err := env.CreatePod(env.Namespace, pod)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)
		err = env.WaitForPodReady(env.Namespace, pod.GetName())
		Expect(err).NotTo(HaveOccurred())
	}

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when entangled pod is created", func() {
		BeforeEach(func() {
			tearDown, err := env.CreateSecret(env.Namespace, env.DefaultQuarksLinkSecret(deploymentName, "nats"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			pod = env.EntangledPod(deploymentName)
		})

		It("mounts the secret on the pod", func() {
			act(pod)

			By("checking the volume and mounts", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.Volumes).To(HaveLen(2))
				Expect(volumeNames(p.Spec.Volumes)).To(ContainElement("link-nats-nats"))

				for _, c := range p.Spec.Containers {
					Expect(c.VolumeMounts).To(HaveLen(2))
					Expect(volumeMountNames(c.VolumeMounts)).To(ContainElement("link-nats-nats"))
				}
			})
		})
	})

	Context("when entangled pod is using multiple links", func() {
		BeforeEach(func() {
			tearDown, err := env.CreateSecret(env.Namespace, env.DefaultQuarksLinkSecret(deploymentName, "nats"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			otherSecret := env.QuarksLinkSecret(
				deploymentName,
				"type",
				"name",
				map[string][]byte{
					"foo":      []byte("[1,2,3]"),
					"password": []byte("abc"),
				},
			)

			tearDown, err = env.CreateSecret(env.Namespace, otherSecret)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			pod = env.EntangledPod(deploymentName)
			pod.Annotations["quarks.cloudfoundry.org/consumes"] = `[{"name":"nats","type":"nats"},{"name":"name","type":"type"}]`
		})

		It("mounts both secrets on the pod", func() {
			act(pod)

			By("checking the volume and mounts", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.Volumes).To(HaveLen(3))
				Expect(volumeNames(p.Spec.Volumes)).To(ContainElement("link-nats-nats"))
				Expect(volumeNames(p.Spec.Volumes)).To(ContainElement("link-type-name"))

				for _, c := range p.Spec.Containers {
					Expect(c.VolumeMounts).To(HaveLen(3))
					mounts := c.VolumeMounts
					Expect(volumeMountNames(mounts)).To(ContainElement("link-nats-nats"))
					Expect(volumeMountNames(mounts)).To(ContainElement("link-type-name"))
				}
			})
		})
	})

	Context("when no entanglement secret can be found", func() {
		BeforeEach(func() {
			pod = env.EntangledPod("non-existent")
		})

		It("refuses to mutate the pod", func() {
			_, err := env.CreatePod(env.Namespace, pod)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`admission webhook "mutate-tangled-pods.quarks.cloudfoundry.org" denied the request: couldn't find any entanglement secret for deployment 'non-existent'`))
		})
	})

	Context("when existing entangled pod is modified", func() {
		BeforeEach(func() {
			pod = env.EntangledPod(deploymentName)

			tearDown, err := env.CreateSecret(env.Namespace, env.DefaultQuarksLinkSecret(deploymentName, "nats"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			tearDown, err = env.CreateSecret(env.Namespace, env.DefaultQuarksLinkSecret(deploymentName, "nuts"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
		})

		It("does not update the volumes and mounts", func() {
			act(pod)

			By("updating the annotations on an existing pod", func() {
				updateEntanglementAnnotations(consumesNuts)
			})

			By("checking volume and mounts stay the same", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.Volumes).To(HaveLen(2))
				Expect(volumeNames(p.Spec.Volumes)).To(ContainElement("link-nats-nats"))

				for _, c := range p.Spec.Containers {
					Expect(c.VolumeMounts).To(HaveLen(2))
					Expect(volumeMountNames(c.VolumeMounts)).To(ContainElement("link-nats-nats"))
				}
			})
		})
	})

	Context("when entanglement annotations are added to normal pod", func() {
		BeforeEach(func() {
			tearDown, err := env.CreateSecret(env.Namespace, env.DefaultQuarksLinkSecret(deploymentName, "nats"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			pod = env.AnnotatedPod("somepod", map[string]string{})
		})

		It("does no modify the pod, because volumes can't be changed", func() {
			act(pod)

			By("updating the annotations on an existing pod", func() {
				updateEntanglementAnnotations(consumesNats)
			})

			By("checking the volume and mounts", func() {
				p, err := env.GetPod(env.Namespace, pod.GetName())
				Expect(err).NotTo(HaveOccurred())

				Expect(p.Spec.Volumes).To(HaveLen(1))
				for _, c := range p.Spec.Containers {
					Expect(c.VolumeMounts).To(HaveLen(1))
				}
			})
		})
	})
})
