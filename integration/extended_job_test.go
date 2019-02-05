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

		Context("when persisting output", func() {
			var (
				oej *ejv1.ExtendedJob
			)

			BeforeEach(func() {
				oej = env.OutputExtendedJob("output-job",
					env.MultiContainerPodTemplate([]string{"echo", `{"foo": "1", "bar": "baz"}`}))
			})

			It("persists output when output peristance is configured", func() {
				_, tearDown, err := env.CreateExtendedJob(env.Namespace, *oej)
				Expect(err).NotTo(HaveOccurred())
				defer tearDown()

				tearDown, err = env.CreatePod(env.Namespace, env.LabeledPod("foo", testLabels("key", "value")))
				Expect(err).NotTo(HaveOccurred())
				defer tearDown()
				err = env.WaitForPods(env.Namespace, "test=true")
				Expect(err).NotTo(HaveOccurred(), "error waiting for pods")

				_, err = env.CollectJobs(env.Namespace, "extendedjob=true", 1)
				Expect(err).NotTo(HaveOccurred())

				By("persisting output for the first container")
				secret, err := env.GetSecret(env.Namespace, "output-job-output-busybox")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(secret.Data["foo"])).To(Equal("1"))
				Expect(string(secret.Data["bar"])).To(Equal("baz"))

				By("persisting output for the second container")
				secret, err = env.GetSecret(env.Namespace, "output-job-output-busybox2")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(secret.Data["foo"])).To(Equal("1"))
				Expect(string(secret.Data["bar"])).To(Equal("baz"))
			})

			Context("when a secret with the same name already exists", func() {
				BeforeEach(func() {
					oej.Spec.Output.NamePrefix = "overwrite-job-output-"
				})

				It("overwrites the secret", func() {
					existingSecret := env.DefaultSecret("overwrite-job-output-busybox")
					existingSecret.StringData["foo"] = "old"
					existingSecret.StringData["bar"] = "old"
					tearDown, err := env.CreateSecret(env.Namespace, existingSecret)
					defer tearDown()
					Expect(err).ToNot(HaveOccurred())

					_, tearDown, err = env.CreateExtendedJob(env.Namespace, *oej)
					Expect(err).NotTo(HaveOccurred())
					defer tearDown()

					tearDown, err = env.CreatePod(env.Namespace, env.LabeledPod("foo", testLabels("key", "value")))
					Expect(err).NotTo(HaveOccurred())
					defer tearDown()

					// Wait until the output of the second container has been persisted. Then check the first one
					_, err = env.GetSecret(env.Namespace, "overwrite-job-output-busybox2")
					secret, err := env.GetSecret(env.Namespace, "overwrite-job-output-busybox")
					Expect(err).ToNot(HaveOccurred())
					Expect(string(secret.Data["foo"])).To(Equal("1"))
					Expect(string(secret.Data["bar"])).To(Equal("baz"))
				})
			})

			Context("when the job failed", func() {
				BeforeEach(func() {
					oej.Spec.Output.NamePrefix = "output-job2-output-"
					oej.Spec.Template = env.FailingMultiContainerPodTemplate([]string{"echo", `{"foo": "1", "bar": "baz"}`})
				})

				Context("and WriteOnFailure is false", func() {
					It("does not persist output", func() {
						_, tearDown, err := env.CreateExtendedJob(env.Namespace, *oej)
						Expect(err).NotTo(HaveOccurred())
						defer tearDown()

						tearDown, err = env.CreatePod(env.Namespace, env.LabeledPod("foo", testLabels("key", "value")))
						Expect(err).NotTo(HaveOccurred())
						defer tearDown()
						err = env.WaitForPods(env.Namespace, "test=true")

						By("not persisting output for the first container")
						_, err = env.GetSecret(env.Namespace, "output-job2-output-busybox")
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("timed out"))
					})
				})

				Context("and WriteOnFailure is true", func() {
					BeforeEach(func() {
						oej.Spec.Output.NamePrefix = "output-job3-output-"
						oej.Spec.Output.WriteOnFailure = true
					})

					It("persists the output", func() {
						_, tearDown, err := env.CreateExtendedJob(env.Namespace, *oej)
						Expect(err).NotTo(HaveOccurred())
						defer tearDown()

						tearDown, err = env.CreatePod(env.Namespace, env.LabeledPod("foo", testLabels("key", "value")))
						Expect(err).NotTo(HaveOccurred())
						defer tearDown()
						err = env.WaitForPods(env.Namespace, "test=true")

						By("persisting the output for the first container")
						_, err = env.GetSecret(env.Namespace, "output-job3-output-busybox")
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})
	})
})
