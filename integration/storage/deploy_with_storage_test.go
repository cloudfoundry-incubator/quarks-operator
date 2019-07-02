package storage_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bm "code.cloudfoundry.org/cf-operator/testing/boshmanifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DeployWithStorage", func() {

	Context("when using multiple processes in BPM", func() {
		It("should add multiple containers to a pod", func() {

			By("Creating a secret for implicit variable")
			storageClass, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS")
			Expect(ok).To(Equal(true))

			tearDown, err := env.CreateSecret(env.Namespace, env.StorageClassSecret("test-bdpl.var-implicit-operator-test-storage-class", storageClass))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "bpm-manifest"},
				Data:       map[string]string{"manifest": bm.BPMRelease},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test-bdpl", "bpm-manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("waiting for deployment to succeed, by checking for a pod")
			err = env.WaitForPod(env.Namespace, "test-bdpl-bpm-v1-0")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from deployment")

			By("checking for services")
			svc, err := env.GetService(env.Namespace, "test-bdpl-bpm")
			Expect(err).NotTo(HaveOccurred(), "error getting service")
			Expect(svc.Spec.Selector).To(Equal(map[string]string{bdm.LabelInstanceGroupName: "bpm"}))
			Expect(svc.Spec.Ports).NotTo(BeEmpty())
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(1337)))
			Expect(svc.Spec.Ports[1].Port).To(Equal(int32(1338)))

			By("checking for containers")
			pods, _ := env.GetPods(env.Namespace, "fissile.cloudfoundry.org/instance-group-name=bpm")
			Expect(len(pods.Items)).To(Equal(1))
			pod := pods.Items[0]
			Expect(pod.Spec.Containers).To(HaveLen(2))

		})
	})

})
