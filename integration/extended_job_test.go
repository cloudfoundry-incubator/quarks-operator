package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ExtendedJob", func() {
	Context("when using label matchers", func() {
		AfterEach(func() {
			env.WaitForPodsDelete(env.Namespace)
		})

		testLabels := func(key, value string) map[string]string {
			labels := map[string]string{key: value, "test": "true"}
			return labels
		}

		It("does not start a job without matches", func() {
			ej := *env.DefaultExtendedJob("extendedjob")
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			jobList, err := env.Clientset.BatchV1().Jobs(env.Namespace).List(metav1.ListOptions{
				LabelSelector: "extendedjob=true",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(jobList.Items).To(HaveLen(0))
		})

		It("should start a job for a matched pod once", func() {
			for _, pod := range []corev1.Pod{
				env.LabeledPod("nomatch", testLabels("key", "nomatch")),
				env.LabeledPod("foo", testLabels("key", "value")),
				env.LabeledPod("bar", testLabels("key", "value")),
			} {
				tearDown, err := env.CreatePod(env.Namespace, pod)
				Expect(err).NotTo(HaveOccurred())
				defer tearDown()
			}

			err := env.WaitForPods(env.Namespace, "test=true")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pods")

			for _, ej := range []ejv1.ExtendedJob{
				*env.DefaultExtendedJob("extendedjob"),
				*env.LongRunningExtendedJob("slowjob"),
				*env.LabelTriggeredExtendedJob(
					"unmatched",
					map[string]string{"unmatched": "unmatched"},
					[]string{"sleep", "1"},
				),
			} {
				_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
				Expect(err).NotTo(HaveOccurred())
				defer tearDown()
			}

			jobs, err := env.CollectJobs(env.Namespace, "extendedjob=true", 4)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(4))
			Expect(env.ContainJob(jobs, "job-extendedjob-foo")).To(Equal(true))
			Expect(env.ContainJob(jobs, "job-slowjob-foo")).To(Equal(true))
			Expect(env.ContainJob(jobs, "job-extendedjob-bar")).To(Equal(true))
			Expect(env.ContainJob(jobs, "job-slowjob-bar")).To(Equal(true))

			By("job does not appear again")

			env.WaitForJobEnd(env.Namespace, "job-extendedjob-foo")
			err = env.WaitForJob(env.Namespace, "job-extendedjob-foo")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
