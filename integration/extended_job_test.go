package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ExtendedJob", func() {
	ownerRef := func(exJob ejv1.ExtendedJob) metav1.OwnerReference {
		return metav1.OwnerReference{
			APIVersion:         "fissile.cloudfoundry.org/v1alpha1",
			Kind:               "ExtendedJob",
			Name:               exJob.Name,
			UID:                exJob.UID,
			Controller:         helper.Bool(true),
			BlockOwnerDeletion: helper.Bool(true),
		}
	}

	Context("when using manually triggered errand job", func() {
		AfterEach(func() {
			env.WaitForPodsDelete(env.Namespace)
		})

		It("does not start a job without Run being set to now", func() {
			ej := env.ErrandExtendedJob("extendedjob")
			ej.Spec.Run = ejv1.RunManually
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			exists, err := env.WaitForJobExists(env.Namespace, "extendedjob=true")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			latest, err := env.GetExtendedJob(env.Namespace, ej.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Run).To(Equal(ejv1.RunManually))

			err = env.UpdateExtendedJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			exists, err = env.WaitForJobExists(env.Namespace, "extendedjob=true")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("starts a job when creating extended job with now", func() {
			ej := env.ErrandExtendedJob("extendedjob")
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			jobs, err := env.CollectJobs(env.Namespace, "extendedjob=true", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(1))
		})

		It("starts a job when updating extended job to now", func() {
			ej := env.ErrandExtendedJob("extendedjob")
			ej.Spec.Run = ejv1.RunManually
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			latest, err := env.GetExtendedJob(env.Namespace, ej.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Run).To(Equal(ejv1.RunManually))

			latest.Spec.Run = ejv1.RunNow
			err = env.UpdateExtendedJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			jobs, err := env.CollectJobs(env.Namespace, "extendedjob=true", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(1))

			Expect(jobs[0].GetOwnerReferences()).Should(ContainElement(ownerRef(*latest)))

		})
	})

	Context("when using label matchers to trigger jobs", func() {
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

			exists, err := env.WaitForJobExists(env.Namespace, "extendedjob=true")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should pick up new extended jobs", func() {
			By("going into reconciliation without extended jobs")
			pod := env.LabeledPod("nomatch", testLabels("key", "nomatch"))
			tearDown, err := env.CreatePod(env.Namespace, pod)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()
			err = env.WaitForPods(env.Namespace, "test=true")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pods")

			By("creating a first extended job")
			ej := *env.DefaultExtendedJob("extendedjob")
			_, tearDown, err = env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			By("triggering another reconciliation")
			pod = env.LabeledPod("foo", testLabels("key", "value"))
			tearDown, err = env.CreatePod(env.Namespace, pod)
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			By("waiting for the job")
			_, err = env.CollectJobs(env.Namespace, "extendedjob=true", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")

			// Why does the ExtendedJob deletion not trigger the job deletion?
			env.DeleteJobs(env.Namespace, "extendedjob=true")
			env.WaitForJobsDeleted(env.Namespace, "extendedjob=true")
		})

		It("should start a job for a matched pod", func() {
			// we have to create jobs first, reconciler noops if no job matches
			By("creating extended jobs")
			for _, ej := range []ejv1.ExtendedJob{
				*env.DefaultExtendedJob("extendedjob"),
				*env.LongRunningExtendedJob("slowjob"),
				*env.LabelTriggeredExtendedJob(
					"unmatched",
					"ready",
					map[string]string{"unmatched": "unmatched"},
					[]string{"sleep", "1"},
				),
			} {
				_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
				Expect(err).NotTo(HaveOccurred())
				defer tearDown()
			}

			By("creating three pods, two match extended jobs and trigger jobs")
			for _, pod := range []corev1.Pod{
				env.LabeledPod("nomatch", testLabels("key", "nomatch")),
				env.LabeledPod("foo", testLabels("key", "value")),
				env.LabeledPod("bar", testLabels("key", "value")),
			} {
				tearDown, err := env.CreatePod(env.Namespace, pod)
				Expect(err).NotTo(HaveOccurred())
				defer tearDown()
			}

			By("waiting for the jobs")
			jobs, err := env.CollectJobs(env.Namespace, "extendedjob=true", 4)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(4))
			Expect(env.ContainJob(jobs, "job-extendedjob-foo")).To(Equal(true))
			Expect(env.ContainJob(jobs, "job-slowjob-foo")).To(Equal(true))
			Expect(env.ContainJob(jobs, "job-extendedjob-bar")).To(Equal(true))
			Expect(env.ContainJob(jobs, "job-slowjob-bar")).To(Equal(true))

			By("checking if owner ref is set")
			latest, err := env.GetExtendedJob(env.Namespace, "extendedjob")
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs[0].GetOwnerReferences()).Should(ContainElement(ownerRef(*latest)))
		})
	})
})
