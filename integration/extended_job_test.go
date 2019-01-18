package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ExtendedJob", func() {
	Context("when using label matchers", func() {
		AfterEach(func() {
			env.WaitForPodsDelete(env.Namespace)
		})

		It("should start a job for a matched pod once", func() {
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, *env.DefaultExtendedJob("extendedjob"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			env.CreatePod(env.Namespace, env.LabeledPod("foo", map[string]string{"key": "value"}))
			err = env.WaitForPod(env.Namespace, "foo")
			defer func() {
				client := env.Clientset.CoreV1().ConfigMaps(env.Namespace)
				client.Delete("foo", &metav1.DeleteOptions{})
			}()

			// check for job
			err = env.WaitForJob(env.Namespace, "job-extendedjob-foo")
			Expect(err).NotTo(HaveOccurred(), "error waiting for job from extendedjob")
		})
	})
})
