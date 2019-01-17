package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExtendedJob", func() {
	Context("when correctly setup", func() {
		AfterEach(func() {
			env.WaitForPodsDelete(env.Namespace)
		})

		It("should start a job", func() {
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, *env.DefaultExtendedJob("extendedjob"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			//env.CreatePod(env.Namespace, env.DefaultPod("foo"))

			// check for job
			//err = env.WaitForJob(env.Namespace, "defaultJob")
			//Expect(err).NotTo(HaveOccurred(), "error waiting for job from extendedjob")
		})
	})
})
