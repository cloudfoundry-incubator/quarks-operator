package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/quarkslink"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("QuarksLink Restart", func() {
	const (
		deploymentName = "nats-deployment"
	)
	var (
		tearDowns []machine.TearDownFunc
		pod       corev1.Pod
		secret    corev1.Secret
	)

	labels := map[string]string{"testselector": "test"}

	BeforeEach(func() {
		secret = env.DefaultQuarksLinkSecret(deploymentName, "nats")
		tearDown, err := env.CreateSecret(env.Namespace, secret)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		pod = env.EntangledPod(deploymentName)
		pod.SetLabels(labels)
	})

	Context("when quarks link secret from entanglement changes", func() {
		JustBeforeEach(func() {
			secret.Data["new.new"] = []byte("eyJrZXkiOiJ2YWx1ZSJ9Cg==")
			_, _, err := env.UpdateSecret(env.Namespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when entangled pod is owned by a statefulset", func() {
			BeforeEach(func() {
				statefulset := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-statefulset",
						Namespace: env.Namespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						Replicas: pointers.Int32(1),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: pod.ObjectMeta,
							Spec:       pod.Spec,
						},
					},
				}
				tearDown, err := env.CreateStatefulSet(env.Namespace, *statefulset)
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)
				err = env.WaitForStatefulSet(env.Namespace, statefulset.Name)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds the restart annotation to the template", func() {
				statefulset, err := env.CollectStatefulSet(env.Namespace, "test-statefulset", 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(statefulset.Generation).To(Equal(int64(2)))
				Expect(statefulset.Spec.Template.GetAnnotations()).To(HaveKey(quarkslink.RestartKey))
			})
		})

		Context("when entangled pod is owned by a deployment", func() {
			BeforeEach(func() {
				dpl := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: env.Namespace,
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						Replicas: pointers.Int32(1),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: pod.ObjectMeta,
							Spec:       pod.Spec,
						},
					},
				}
				tearDown, err := env.CreateDeployment(env.Namespace, *dpl)
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)
				err = env.WaitForDeployment(env.Namespace, dpl.Name, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds the restart annotation to the template", func() {
				dpl, err := env.CollectDeployment(env.Namespace, "test-deployment", 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(dpl.Generation).To(Equal(int64(2)))
				Expect(dpl.Spec.Template.GetAnnotations()).To(HaveKey(quarkslink.RestartKey))
			})
		})
	})
})
