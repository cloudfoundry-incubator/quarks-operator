package storage_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DeployWithStorage", func() {
	Context("when using multiple processes in BPM", func() {
		It("should add multiple containers to a pod", func() {
			By("Creating a secret for implicit variable")
			storageClass, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS")
			Expect(ok).To(Equal(true))

			tearDown, err := env.CreateSecret(env.Namespace, env.StorageClassSecret("var-operator-test-storage-class", storageClass))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "bpm-manifest"},
				Data:       map[string]string{"manifest": bm.BPMRelease},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test-bdpl", "bpm-manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test-bdpl", "bpm", "1", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for services")
			svc, err := env.GetService(env.Namespace, "bpm")
			Expect(err).NotTo(HaveOccurred(), "error getting service")
			Expect(svc.Spec.Selector).To(Equal(map[string]string{
				bdv1.LabelInstanceGroupName: "bpm",
				bdv1.LabelDeploymentName:    "test-bdpl",
			}))
			Expect(svc.Spec.Ports).NotTo(BeEmpty())
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(1337)))
			Expect(svc.Spec.Ports[1].Port).To(Equal(int32(1338)))

			By("checking for containers")
			pods, _ := env.GetPods(env.Namespace, "quarks.cloudfoundry.org/instance-group-name=bpm")
			Expect(len(pods.Items)).To(Equal(1))
			pod := pods.Items[0]
			Expect(pod.Spec.Containers).To(HaveLen(3))

		})
	})

	Context("when specifying affinity", func() {
		sts1Name := "bpm1"
		sts2Name := "bpm2"
		sts3Name := "bpm3"

		It("should create available resources", func() {
			nodes, err := env.GetNodes()
			Expect(err).NotTo(HaveOccurred(), "error getting nodes")
			if len(nodes) < 2 {
				Skip("Skipping because nodes is less than 2")
			}

			By("Creating a secret for implicit variable")
			storageClass, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS")
			Expect(ok).To(Equal(true))

			tearDown, err := env.CreateSecret(env.Namespace, env.StorageClassSecret("var-operator-test-storage-class", storageClass))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap("bpm-affinity", bm.BPMReleaseWithAffinity))
			Expect(err).NotTo(HaveOccurred(), "error creating configMap")
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("bpm-affinity", "bpm-affinity"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for pod")
			err = env.WaitForInstanceGroup(env.Namespace, "bpm-affinity", "bpm1", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for pods from instance group bpm1")
			err = env.WaitForInstanceGroup(env.Namespace, "bpm-affinity", "bpm2", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for pods from instance group bpm2")
			err = env.WaitForInstanceGroup(env.Namespace, "bpm-affinity", "bpm3", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for pods from instance group bpm3")

			By("checking for affinity")
			sts1, err := env.GetStatefulSet(env.Namespace, sts1Name)
			Expect(err).NotTo(HaveOccurred(), "error getting statefulset for deployment")
			Expect(sts1.Spec.Template.Spec.Affinity.NodeAffinity).To(Equal(&corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "beta.kubernetes.io/os",
									Operator: "In",
									Values: []string{
										"linux",
										"darwin",
									},
								},
							},
						},
					},
				},
			}))

			sts2, err := env.GetStatefulSet(env.Namespace, sts2Name)
			Expect(err).NotTo(HaveOccurred(), "error getting statefulset for deployment")
			Expect(sts2.Spec.Template.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "instance-name",
									Operator: "In",
									Values: []string{
										"bpm2",
									},
								},
							},
						},
						TopologyKey: "beta.kubernetes.io/os",
					},
				},
			}))

			sts3, err := env.GetStatefulSet(env.Namespace, sts3Name)
			Expect(err).NotTo(HaveOccurred(), "error getting statefulset for deployment")
			Expect(sts3.Spec.Template.Spec.Affinity.PodAntiAffinity).To(Equal(&corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "instance-name",
										Operator: "In",
										Values: []string{
											"bpm3",
										},
									},
								},
							},
							TopologyKey: "beta.kubernetes.io/os",
						},
					},
				},
			}))
		})
	})

})
