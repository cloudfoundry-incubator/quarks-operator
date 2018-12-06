package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deploy", func() {
	Context("when correctly setup", func() {
		podName := "diego-pod"

		AfterEach(func() {
			err := env.WaitForPodsDelete(env.Namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should deploy a pod", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.DefaultFissileCR("test", "manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			// check for pod
			err = env.WaitForPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from deployment")
		})

		It("should deploy manifest with multiple ops correctly", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			tearDown, err = env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			tearDown, err = env.CreateSecret(env.Namespace, env.InterpolateOpsSecret("bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.InterpolateFissileCR("test", "manifest", "bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			// check for pod
			err = env.WaitForPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from deployment")
			labeled, err := env.PodLabeled(env.Namespace, podName, "size", "4")
			Expect(labeled).To(BeTrue())
			Expect(err).NotTo(HaveOccurred(), "error verifying pod label")
		})
	})

	Context("when incorrectly setup", func() {
		It("failed to deploy a empty manifest", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.EmptyFissileCR("test", "manifest"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.manifest.type in body should be one of"))
			Expect(err.Error()).To(ContainSubstring("spec.manifest.ref in body should be at least 1 chars long"))
			defer tearDown()
		})

		It("failed to deploy due to a wrong manifest type", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.WrongTypeFissileCR("test", "manifest"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.manifest.type in body should be one of"))
			defer tearDown()
		})

		It("failed to deploy due to a empty manifest ref", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.DefaultFissileCR("test", ""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.manifest.ref in body should be at least 1 chars long"))
			defer tearDown()
		})

		It("failed to deploy due to a wrong ops type", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.FissileCRWithWrongTypeOps("test", "manifest", "ops"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.ops.type in body should be one of"))
			defer tearDown()
		})

		It("failed to deploy due to a empty ops ref", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifest("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			_, tearDown, err = env.CreateFissileCR(env.Namespace, env.DefaultFissileCRWithOps("test", "manifest", ""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.ops.ref in body should be at least 1 chars long"))
			defer tearDown()
		})
	})
})
