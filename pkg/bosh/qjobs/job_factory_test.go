package qjobs_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	. "code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/qjobs"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/testing"
)

var _ = Describe("JobFactory", func() {
	var (
		factory        *qjobs.JobFactory
		deploymentName string
		m              *manifest.Manifest
		env            testing.Catalog
		err            error
		linkInfos      LinkInfos
	)

	BeforeEach(func() {
		deploymentName = "foo-deployment"
		m, err = env.DefaultBOSHManifest()
		linkInfos = LinkInfos{}
		Expect(err).NotTo(HaveOccurred())
		factory = qjobs.NewJobFactory()
	})

	Describe("InstanceGroupManifestJob", func() {
		It("creates init containers", func() {
			qJob, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
			Expect(err).ToNot(HaveOccurred())
			jobIG := qJob.Spec.Template.Spec
			// Test init containers in the ig manifest qJob
			Expect(jobIG.Template.Spec.InitContainers[0].Name).To(Equal("spec-copier-redis"))
			Expect(jobIG.Template.Spec.InitContainers[1].Name).To(Equal("spec-copier-cflinuxfs3"))
			Expect(jobIG.Template.Spec.InitContainers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
			Expect(jobIG.Template.Spec.InitContainers[1].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
		})

		Context("when link infos are provided", func() {
			var jobIG batchv1.JobSpec

			BeforeEach(func() {
				linkInfos = LinkInfos{
					{
						SecretName:   "fake-secret-name",
						ProviderName: "fake-link-name",
					},
				}
				qJob, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
				Expect(err).ToNot(HaveOccurred())
				jobIG = qJob.Spec.Template.Spec
			})

			It("keeps existing volume mounts on init containers", func() {
				Expect(jobIG.Template.Spec.InitContainers[0].Name).To(Equal("spec-copier-redis"))
				Expect(jobIG.Template.Spec.InitContainers[1].Name).To(Equal("spec-copier-cflinuxfs3"))
				Expect(jobIG.Template.Spec.InitContainers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
				Expect(jobIG.Template.Spec.InitContainers[1].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
			})

			It("adds the linked secret as a volume and mounts it on the first container", func() {
				Expect(jobIG.Template.Spec.Volumes[0].Name).To(Equal("fake-secret-name"))
				Expect(jobIG.Template.Spec.Volumes[0].Secret.SecretName).To(Equal("fake-secret-name"))
				Expect(jobIG.Template.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("fake-secret-name"))
				Expect(jobIG.Template.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal(VolumeLinksPath + "fake-link-name"))
			})
		})

		It("handles an error when getting release image", func() {
			m.Stemcells = nil
			_, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generation of gathering job 'redis-server' failed for instance group"))
		})

		It("does not generate the instance group containers when its instances is zero", func() {
			m.InstanceGroups[0].Instances = 0
			qJob, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
			Expect(err).ToNot(HaveOccurred())
			jobIG := qJob.Spec.Template.Spec
			Expect(len(jobIG.Template.Spec.InitContainers)).To(BeNumerically("<", 2))
			Expect(len(jobIG.Template.Spec.Containers)).To(BeNumerically("<", 2))
		})

		Context("when manifest contains links", func() {
			It("creates output entries for all provides", func() {
				m, err = env.ElaboratedBOSHManifest()
				Expect(err).NotTo(HaveOccurred())
				qJob, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
				Expect(err).ToNot(HaveOccurred())
				om := qJob.Spec.Output.OutputMap
				Expect(om).To(Equal(
					qjv1a1.OutputMap{
						"redis-slave": qjv1a1.FilesToSecrets{
							"ig.json": qjv1a1.SecretOptions{
								Name: "ig-resolved.redis-slave",
								AdditionalSecretLabels: map[string]string{
									"quarks.cloudfoundry.org/entanglement": "true",
									"quarks.cloudfoundry.org/secret-type":  "ig-resolved",
								},
								AdditionalSecretAnnotations: map[string]string{},
								Versioned:                   true,
								PersistenceMethod:           "",
							},
							"bpm.json": qjv1a1.SecretOptions{
								Name: "bpm.redis-slave",
								AdditionalSecretLabels: map[string]string{
									"quarks.cloudfoundry.org/entanglement": "true",
									"quarks.cloudfoundry.org/secret-type":  "bpm",
								},
								AdditionalSecretAnnotations: map[string]string{},
								Versioned:                   true,
								PersistenceMethod:           "",
							},
							"provides.json": qjv1a1.SecretOptions{
								Name: "link",
								AdditionalSecretLabels: map[string]string{
									"quarks.cloudfoundry.org/entanglement": "true",
								},
								AdditionalSecretAnnotations: map[string]string{},
								Versioned:                   false,
								PersistenceMethod:           "fan-out",
							},
						},
						"diego-cell": qjv1a1.FilesToSecrets{
							"ig.json": qjv1a1.SecretOptions{
								Name: "ig-resolved.diego-cell",
								AdditionalSecretLabels: map[string]string{
									"quarks.cloudfoundry.org/entanglement": "true",
									"quarks.cloudfoundry.org/secret-type":  "ig-resolved",
								},
								AdditionalSecretAnnotations: map[string]string{},
								Versioned:                   true,
								PersistenceMethod:           "",
							},
							"bpm.json": qjv1a1.SecretOptions{
								Name: "bpm.diego-cell",
								AdditionalSecretLabels: map[string]string{
									"quarks.cloudfoundry.org/entanglement": "true",
									"quarks.cloudfoundry.org/secret-type":  "bpm",
								},
								AdditionalSecretAnnotations: map[string]string{},
								Versioned:                   true,
								PersistenceMethod:           "",
							},
							"provides.json": qjv1a1.SecretOptions{
								Name: "link",
								AdditionalSecretLabels: map[string]string{
									"quarks.cloudfoundry.org/entanglement": "true",
								},
								AdditionalSecretAnnotations: map[string]string{},
								Versioned:                   false,
								PersistenceMethod:           "fan-out",
							},
						},
					},
				))
			})
		})

		It("has one spec-copier init container per instance group", func() {
			job, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
			Expect(err).ToNot(HaveOccurred())

			spec := job.Spec.Template.Spec.Template.Spec
			Expect(job.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentName, deploymentName))

			Expect(len(spec.InitContainers)).To(Equal(len(m.InstanceGroups)))
			Expect(spec.InitContainers[0].Name).To(ContainSubstring("spec-copier-"))
		})

		It("has one bpm-configs container per instance group", func() {
			job, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
			Expect(err).ToNot(HaveOccurred())

			spec := job.Spec.Template.Spec.Template.Spec
			Expect(len(spec.Containers)).To(Equal(len(m.InstanceGroups)))
			Expect(spec.Containers[0].Name).To(Equal(m.InstanceGroups[0].Name))
			Expect(spec.Containers[0].Args).To(Equal([]string{"util", "instance-group", "--initial-rollout", "true"}))
		})

		It("does not generate the instance group containers when its instances is zero", func() {
			m.InstanceGroups[0].Instances = 0
			job, err := factory.InstanceGroupManifestJob("namespace", deploymentName, *m, linkInfos, true)
			Expect(err).ToNot(HaveOccurred())

			spec := job.Spec.Template.Spec.Template.Spec
			Expect(len(spec.InitContainers)).To(BeNumerically("<", 2))
			Expect(len(spec.Containers)).To(BeNumerically("<", 2))
		})

	})
})
