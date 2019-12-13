package manifest_test

import (
	"encoding/json"
	"fmt"

	"github.com/go-test/deep"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("InstanceGroupResolver", func() {

	var (
		m   *Manifest
		env testing.Catalog
		igr *InstanceGroupResolver
		ig  string
		err error
	)

	Context("Job", func() {
		Describe("property helper to override job specs from manifest", func() {
			It("should find a property value in the manifest job properties section (constructed example)", func() {
				// health.disk.warning
				exampleJob := Job{
					Properties: JobProperties{
						Properties: map[string]interface{}{
							"health": map[interface{}]interface{}{
								"disk": map[interface{}]interface{}{
									"warning": 42,
								},
							},
						},
					},
				}

				value, ok := exampleJob.Property("health.disk.warning")
				Expect(ok).To(BeTrue())
				Expect(value).To(BeEquivalentTo(42))

				value, ok = exampleJob.Property("health.disk.nonexisting")
				Expect(ok).To(BeFalse())
				Expect(value).To(BeNil())
			})

			It("should find a property value in the manifest job properties section (proper manifest example)", func() {
				m, err = env.BOSHManifestWithProviderAndConsumer()
				Expect(err).NotTo(HaveOccurred())
				job := m.InstanceGroups[0].Jobs[0]

				value, ok := job.Property("doppler.grpc_port")
				Expect(ok).To(BeTrue())
				Expect(value).To(BeEquivalentTo(json.Number("7765")))
			})
		})
	})

	Context("InstanceGroupResolver", func() {
		var fs = afero.NewMemMapFs()

		JustBeforeEach(func() {
			var err error
			igr, err = NewInstanceGroupResolver(fs, assetPath, *m, ig)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("BPMInfo", func() {
			BeforeEach(func() {
				m, err = env.BOSHManifestWithProviderAndConsumer()
				Expect(err).NotTo(HaveOccurred())
				ig = "log-api"
			})

			It("it should have info about instances, azs", func() {
				bpmInfo, err := igr.BPMInfo(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(bpmInfo).ToNot(BeNil())
				Expect(bpmInfo.InstanceGroup.Name).To(Equal("log-api"))
				Expect(bpmInfo.InstanceGroup.Instances).To(Equal(2))
				Expect(bpmInfo.InstanceGroup.AZs).To(Equal([]string{"z1", "z2"}))
			})

			It("returns the bpm config for all jobs", func() {
				bpmInfo, err := igr.BPMInfo(true)
				Expect(err).ToNot(HaveOccurred())

				bpm := bpmInfo.Configs["loggregator_trafficcontroller"]
				Expect(bpm).ToNot(BeNil())
				Expect(bpm.Processes[0].Executable).To(Equal("/var/vcap/packages/loggregator_trafficcontroller/trafficcontroller"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKADDRESS"]).To(Equal("cf-doppler"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKVALUES"]).To(Equal("10001"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKNESTEDVALUES"]).To(Equal("7765"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKINSTANCESAZ"]).To(Equal("z1"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKINSTANCESADDRESS"]).To(Equal("cf-doppler-0"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECADDRESS"]).To(Equal("cf-log-api-0"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECDEPLOYMENT"]).To(Equal("cf"))

				// The following block of assertions are related to the usage of
				// the BPM spec object instance in an ERB test file, see
				// https://bosh.io/docs/jobs/#properties-spec
				//
				// When rendering the bpm.yml.erb file, the values for an spec object, will
				// always use the instance at index 0 of Properties.Quarks.Instances
				// We do not support different indexes in BPM data.
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECAZ"]).To(Equal("z1"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECBOOTSTRAP"]).To(Equal("true"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECID"]).To(Equal("log-api-0"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECINDEX"]).To(Equal("0"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECNAME"]).To(Equal("log-api-loggregator_trafficcontroller"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECNETWORKS"]).To(Equal(""))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECADDRESS"]).To(Equal("cf-log-api-0"))
			})

			Context("when manifest presets overridden bpm info", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithOverriddenBPMInfo()
					Expect(err).NotTo(HaveOccurred())
					ig = "redis-slave"
				})

				It("returns overwritten bpm config", func() {
					bpmInfo, err := igr.BPMInfo(true)
					Expect(err).ToNot(HaveOccurred())

					bpm := bpmInfo.Configs["redis-server"]
					Expect(bpm).ToNot(BeNil())
					Expect(bpm.Processes[0].Executable).To(Equal("/another/command"))
				})
			})

			Context("when manifest presets absent bpm info", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithAbsentBPMInfo()
					Expect(err).NotTo(HaveOccurred())
					ig = "redis-slave"
				})

				It("returns merged bpm config", func() {
					bpmInfo, err := igr.BPMInfo(true)
					Expect(err).ToNot(HaveOccurred())

					bpm := bpmInfo.Configs["redis-server"]
					Expect(bpm).ToNot(BeNil())
					Expect(bpm.Processes[0].Executable).To(Equal("/var/vcap/packages/redis-4/bin/redis-server"))
					Expect(bpm.Processes[1].Name).To(Equal("absent-process"))
					Expect(bpm.Processes[1].Executable).To(Equal("/absent-process-command"))
				})
			})

			Context("when manifest presets zero instances for the job", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithZeroInstances()
					Expect(err).NotTo(HaveOccurred())
					ig = "nats"
				})

				It("reports an error if the job had empty bpm configs", func() {
					_, err := igr.BPMInfo(true)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Empty bpm configs about job '%s'", ig)))
				})
			})

			Context("with process resources", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithResources()
					Expect(err).NotTo(HaveOccurred())
					ig = "doppler"
				})

				It("returns the bpm with resources for process", func() {
					bpmInfo, err := igr.BPMInfo(true)
					Expect(err).ToNot(HaveOccurred())

					bpm := bpmInfo.Configs["doppler"]
					Expect(bpm).NotTo(BeNil())
					Expect(bpm.Processes[0].Requests.Memory().String()).To(Equal("128Mi"))
					Expect(bpm.Processes[0].Requests.Cpu().String()).To(Equal("5m"))
				})
			})
			Context("with process resources", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithResources()
					Expect(err).NotTo(HaveOccurred())
					ig = "log-api"
				})

				It("raises an error for bpm process without executable", func() {
					_, err := igr.BPMInfo(true)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid BPM process"))
				})
			})
		})

		Describe("Manifest", func() {
			Context("when manifest has multiple instances", func() {
				BeforeEach(func() {
					m, err = env.ElaboratedBOSHManifest()
					Expect(err).NotTo(HaveOccurred())
					ig = "redis-slave"
				})

				It("should remove info about job instances, instance count, azs", func() {
					manifest, err := igr.Manifest(true)
					Expect(err).ToNot(HaveOccurred())

					Expect(manifest.InstanceGroups[0].Jobs[0].Properties.Quarks.Instances).To(BeNil())
					Expect(manifest.InstanceGroups[0].Instances).To(Equal(0))
					Expect(manifest.InstanceGroups[0].AZs).To(BeNil())
				})
			})

			Context("when resolving links between providers and consumers", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithProviderAndConsumer()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when the job consumes a link", func() {
					BeforeEach(func() {
						ig = "log-api"
					})

					It("resolves all required data if the job consumes a link", func() {
						m, err := igr.Manifest(true)
						Expect(err).ToNot(HaveOccurred())

						instanceGroup, ok := m.InstanceGroups.InstanceGroupByName(ig)
						Expect(ok).To(BeTrue())

						jobQuarksConsumes := instanceGroup.Jobs[0].Properties.Quarks.Consumes
						jobConsumesFromDoppler, consumeFromDopplerExists := jobQuarksConsumes["doppler"]
						Expect(consumeFromDopplerExists).To(BeTrue())
						expectedProperties := JobLinkProperties{
							"doppler": map[string]interface{}{
								"grpc_port": json.Number("7765"),
								"fooprop":   json.Number("10001"),
							},
						}

						Expect(deep.Equal(jobConsumesFromDoppler.Properties, expectedProperties)).To(HaveLen(0))
					})
				})

				Context("when the job does not consume a link", func() {
					BeforeEach(func() {
						ig = "doppler"
					})
					It("has an empty consumes list if the job does not consume a link", func() {
						m, err := igr.Manifest(true)
						Expect(err).ToNot(HaveOccurred())

						ig, ok := m.InstanceGroups.InstanceGroupByName(ig)
						Expect(ok).To(BeTrue())

						jobQuarksConsumes := ig.Jobs[0].Properties.Quarks.Consumes
						var emptyJobQuarksConsumes map[string]JobLink
						Expect(jobQuarksConsumes).To(BeEquivalentTo(emptyJobQuarksConsumes))
					})
				})
			})

			Context("when specifying consume as nil", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithNilConsume()
					Expect(err).NotTo(HaveOccurred())
					ig = "log-api"
				})

				It("resolves all required data if the job consumes a link", func() {
					manifest, err := igr.Manifest(true)
					Expect(err).ToNot(HaveOccurred())

					// log-api instance_group, with loggregator_trafficcontroller job, consumes nil link from log-cache
					jobQuarksConsumes := manifest.InstanceGroups[0].Jobs[0].Properties.Quarks.Consumes
					jobConsumesFromLogCache, consumeFromLogCacheExists := jobQuarksConsumes["log-cache"]
					Expect(consumeFromLogCacheExists).To(BeTrue())
					Expect(jobConsumesFromLogCache).To(Equal(JobLink{}))
				})
			})
		})

		Describe("SaveLinks", func() {
			Context("when jobs provide links", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithLinks()
					Expect(err).NotTo(HaveOccurred())
					ig = "nats"
				})

				It("stores all the links of the instance group in a file", func() {
					_, err := igr.Manifest(true)
					Expect(err).ToNot(HaveOccurred())
					err = igr.SaveLinks("/mnt/quarks")
					Expect(err).ToNot(HaveOccurred())

					Expect(afero.Exists(fs, "/mnt/quarks/provides.json")).To(BeTrue())
				})
			})
		})

		Describe("CollectQuarksLinks", func() {
			Context("when jobs provide links", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithExternalLinks()
					Expect(err).NotTo(HaveOccurred())
					ig = "log-api"

					fileP1, err := fs.Create(converter.VolumeLinksPath + "doppler/fooprop")
					Expect(err).NotTo(HaveOccurred())
					defer fileP1.Close()
					_, err = fileP1.WriteString("fake_prop")
					Expect(err).NotTo(HaveOccurred())

					fileP2, err := fs.Create(converter.VolumeLinksPath + "doppler/grpc_port")
					Expect(err).NotTo(HaveOccurred())
					defer fileP2.Close()
					_, err = fileP2.WriteString(`7765`)
					Expect(err).NotTo(HaveOccurred())

				})

				It("stores all the links of the instance group in a file", func() {
					err = igr.CollectQuarksLinks(converter.VolumeLinksPath)
					Expect(err).ToNot(HaveOccurred())

					m, err := igr.Manifest(true)
					Expect(err).ToNot(HaveOccurred())
					// log-api instance_group, with loggregator_trafficcontroller job, consumes links from external doppler
					jobQuarksConsumes := m.InstanceGroups[0].Jobs[0].Properties.Quarks.Consumes
					Expect(jobQuarksConsumes).To(ContainElement(JobLink{
						Address: "doppler-0.default.svc.cluster.local",
						Instances: []JobInstance{
							{
								Address:   "172.30.10.1",
								Name:      "doppler",
								ID:        "pod-uuid",
								Index:     0,
								Bootstrap: true,
							},
						},
						Properties: JobLinkProperties{
							"doppler": map[string]interface{}{
								"grpc_port": "7765",
								"fooprop":   "fake_prop",
							},
						},
					}))

				})
			})
		})
	})
})
