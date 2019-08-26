package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ExtendedJob", func() {
	jobOwnerRef := func(eJob ejv1.ExtendedJob) metav1.OwnerReference {
		return metav1.OwnerReference{
			APIVersion:         "fissile.cloudfoundry.org/v1alpha1",
			Kind:               "ExtendedJob",
			Name:               eJob.Name,
			UID:                eJob.UID,
			Controller:         util.Bool(true),
			BlockOwnerDeletion: util.Bool(true),
		}
	}

	AfterEach(func() {
		env.FlushLog()
	})

	Context("when using an AutoErrandJob", func() {

		var (
			ej        ejv1.ExtendedJob
			tearDowns []environment.TearDownFunc
		)

		BeforeEach(func() {
			ej = env.AutoErrandExtendedJob("autoerrand-job")
		})

		AfterEach(func() {
			Expect(env.TearDownAll(tearDowns)).To(Succeed())
		})

		It("immediately starts the job", func() {
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(1))
		})

		Context("when the job succeeded", func() {
			It("cleans up job immediately", func() {
				_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
				Expect(err).NotTo(HaveOccurred())
				defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

				jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
				Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
				Expect(jobs).To(HaveLen(1))

				err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking pod is still there, because delete label is missing")
				Expect(env.PodsDeleted(env.Namespace)).To(BeFalse())
			})

			Context("when pod template has delete label", func() {
				Context("when delete is set to pod", func() {
					BeforeEach(func() {
						ej.Spec.Template.Labels = map[string]string{"delete": "pod"}
					})

					It("removes job's pod", func() {
						_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
						Expect(err).NotTo(HaveOccurred())
						defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

						jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
						Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
						Expect(jobs).To(HaveLen(1))

						err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
						Expect(err).ToNot(HaveOccurred())

						Expect(env.PodsDeleted(env.Namespace)).To(BeTrue())
					})
				})

				Context("when delete is set to something else", func() {
					BeforeEach(func() {
						ej.Spec.Template.Labels = map[string]string{"delete": "something-else"}
					})

					It("keeps the job's pod", func() {
						_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
						Expect(err).NotTo(HaveOccurred())
						defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

						jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
						Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
						Expect(jobs).To(HaveLen(1))

						err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
						Expect(err).ToNot(HaveOccurred())

						Expect(env.PodsDeleted(env.Namespace)).To(BeFalse())
					})
				})
			})
		})

		Context("when the job failed", func() {
			BeforeEach(func() {
				ej.Spec.Template = env.FailingMultiContainerPodTemplate([]string{"echo", "{}"})
			})

			It("cleans it up when the ExtendedJob is deleted", func() {
				_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
				Expect(err).NotTo(HaveOccurred())
				defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

				jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
				Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
				Expect(jobs).To(HaveLen(1))

				err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
				Expect(err).To(HaveOccurred())

				Expect(tearDown()).To(Succeed())
				err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when configured to update on config change", func() {
			var (
				configMap  corev1.ConfigMap
				secret     corev1.Secret
				tearDownEJ environment.TearDownFunc
			)

			BeforeEach(func() {
				ej.Spec.UpdateOnConfigChange = true
				ej.Spec.Template = env.ConfigPodTemplate()

				configMap = env.DefaultConfigMap("config1")
				secret = env.DefaultSecret("secret1")

				tearDown, err := env.CreateConfigMap(env.Namespace, configMap)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				tearDown, err = env.CreateSecret(env.Namespace, secret)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				_, tearDownEJ, err = env.CreateExtendedJob(env.Namespace, ej)
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDownEJ)

				_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when a config content changes", func() {
				It("it creates a new job", func() {
					By("checking if ext job is done")
					eJob, err := env.GetExtendedJob(env.Namespace, ej.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))
					Expect(env.WaitForLogMsg(env.ObservedLogs, "Deleting succeeded job")).ToNot(HaveOccurred())

					By("modifying config")
					c, _ := env.GetConfigMap(env.Namespace, configMap.Name)
					c.Data["fake-key"] = "fake-value"
					_, _, err = env.UpdateConfigMap(env.Namespace, *c)
					Expect(err).NotTo(HaveOccurred())

					By("checking if job is running")
					jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
					Expect(err).NotTo(HaveOccurred())
					Expect(jobs).To(HaveLen(1))
				})
			})
		})

		Context("when enabling update on config change", func() {
			var (
				configMap  corev1.ConfigMap
				secret     corev1.Secret
				tearDownEJ environment.TearDownFunc
			)

			BeforeEach(func() {
				ej.Spec.UpdateOnConfigChange = false
				ej.Spec.Template = env.ConfigPodTemplate()

				configMap = env.DefaultConfigMap("config1")
				secret = env.DefaultSecret("secret1")

				tearDown, err := env.CreateConfigMap(env.Namespace, configMap)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				tearDown, err = env.CreateSecret(env.Namespace, secret)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				_, tearDownEJ, err = env.CreateExtendedJob(env.Namespace, ej)
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDownEJ)

				_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when referenced configs are created after the extended job", func() {
			var (
				configMap  corev1.ConfigMap
				secret     corev1.Secret
				tearDownEJ environment.TearDownFunc
				tearDown   environment.TearDownFunc
			)

			BeforeEach(func() {
				ej.Spec.UpdateOnConfigChange = true
				ej.Spec.Template = env.ConfigPodTemplate()

				configMap = env.DefaultConfigMap("config1")
				secret = env.DefaultSecret("secret1")

			})

			Context("when the extended job is created after the config map", func() {
				BeforeEach(func() {
					var err error
					tearDown, err = env.CreateSecret(env.Namespace, secret)
					Expect(err).ToNot(HaveOccurred())

					_, tearDownEJ, err = env.CreateExtendedJob(env.Namespace, ej)
					Expect(err).NotTo(HaveOccurred())
				})

				It("the job starts", func() {
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDownEJ)

					By("creating the config map")
					tearDown, err := env.CreateConfigMap(env.Namespace, configMap)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					By("waiting for the job to start")
					_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the extended job is created after the secret", func() {
				BeforeEach(func() {
					var err error
					tearDown, err = env.CreateConfigMap(env.Namespace, configMap)
					Expect(err).ToNot(HaveOccurred())

					_, tearDownEJ, err = env.CreateExtendedJob(env.Namespace, ej)
					Expect(err).NotTo(HaveOccurred())
				})

				It("the job starts", func() {
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDownEJ)

					By("creating the secret")
					tearDown, err := env.CreateSecret(env.Namespace, secret)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					By("waiting for the job to start")
					_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the extended job is created after several configs", func() {
				BeforeEach(func() {
					var err error
					_, tearDownEJ, err = env.CreateExtendedJob(env.Namespace, ej)
					Expect(err).NotTo(HaveOccurred())
				})

				It("the job starts", func() {
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDownEJ)

					By("creating the configs")
					tearDown, err := env.CreateSecret(env.Namespace, secret)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					tearDown, err = env.CreateConfigMap(env.Namespace, configMap)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					By("waiting for the job to start")
					_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
					Expect(err).ToNot(HaveOccurred())
				})
			})

		})

	})

	Context("when using manually triggered ErrandJob", func() {
		It("does not start a job without Run being set to now", func() {
			ej := env.ErrandExtendedJob("extendedjob")
			ej.Spec.Trigger.Strategy = ejv1.TriggerManual
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			exists, err := env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			latest, err := env.GetExtendedJob(env.Namespace, ej.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerManual))

			err = env.UpdateExtendedJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			exists, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob))
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("starts a job when creating extended job with now", func() {
			ej := env.ErrandExtendedJob("extendedjob")
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(1))
		})

		It("starts a job when updating extended job to now", func() {
			ej := env.ErrandExtendedJob("extendedjob")
			ej.Spec.Trigger.Strategy = ejv1.TriggerManual
			_, tearDown, err := env.CreateExtendedJob(env.Namespace, ej)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			latest, err := env.GetExtendedJob(env.Namespace, ej.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerManual))

			latest.Spec.Trigger.Strategy = ejv1.TriggerNow
			err = env.UpdateExtendedJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", ejv1.LabelExtendedJob), 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from extendedjob")
			Expect(jobs).To(HaveLen(1))

			Expect(jobs[0].GetOwnerReferences()).Should(ContainElement(jobOwnerRef(*latest)))

		})
	})

})
