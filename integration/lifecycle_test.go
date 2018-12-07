package integration_test

import (
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeploymentcontroller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lifecycle", func() {
	var (
		fissileCR   bdcv1.BOSHDeployment
		newManifest corev1.ConfigMap
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			fissileCR = env.DefaultFissileCR("testcr", "manifest")
			newManifest = corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{Name: "newmanifest"},
				Data: map[string]string{
					"manifest": `instance-groups:
- name: updated
  instances: 1
`,
				},
			}
		})

		It("should exercise a deployment lifecycle", func() {
			// Create BOSH manifest in config map
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			// Create fissile custom resource
			var versionedCR *bdcv1.BOSHDeployment
			versionedCR, tearDown, err = env.CreateFissileCR(env.Namespace, fissileCR)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			err = env.WaitForPod(env.Namespace, "diego-pod")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from initial deployment")

			// Update
			tearDown, err = env.CreateConfigMap(env.Namespace, newManifest)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			versionedCR.Spec.Manifest.Ref = "newmanifest"
			_, _, err = env.UpdateFissileCR(env.Namespace, *versionedCR)
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPod(env.Namespace, "updated-pod")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from updated deployment")

			// TODO after update we still have diego-pod around
			Expect(env.PodRunning(env.Namespace, "diego-pod")).To(BeTrue())
			Expect(env.PodRunning(env.Namespace, "updated-pod")).To(BeTrue())

			// Delete custom resource
			err = env.DeleteFissileCR(env.Namespace, "testcr")
			Expect(err).NotTo(HaveOccurred(), "error deleting custom resource")
			err = env.WaitForCRDeletion(env.Namespace, "testcr")
			Expect(err).NotTo(HaveOccurred(), "error waiting for custom resource deletion")

			// Deletion of CRD generated request
			Expect(env.Logger.AllMessages()).Should(ContainElement(ContainSubstring("Skip reconcile: CRD not found\n")))

		})
	})

})
