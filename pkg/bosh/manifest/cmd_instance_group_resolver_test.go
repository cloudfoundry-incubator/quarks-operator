package manifest_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/go-test/deep"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("InstanceGroupResolver", func() {

	var (
		m   *Manifest
		env testing.Catalog
		dg  *InstanceGroupResolver
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
		JustBeforeEach(func() {
			var err error
			dg, err = NewInstanceGroupResolver(assetPath, "default", *m, ig)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("BPMConfig", func() {
			BeforeEach(func() {
				m, err = env.BOSHManifestWithProviderAndConsumer()
				Expect(err).NotTo(HaveOccurred())
				ig = "log-api"
			})

			It("returns the bpm config for all jobs", func() {
				bpmConfigs, err := dg.BPMConfigs()
				Expect(err).ToNot(HaveOccurred())

				bpm := bpmConfigs["loggregator_trafficcontroller"]
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
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECID"]).To(Equal("log-api-0-loggregator_trafficcontroller"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECINDEX"]).To(Equal("0"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECNAME"]).To(Equal("log-api-loggregator_trafficcontroller"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECNETWORKS"]).To(Equal(""))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECADDRESS"]).To(Equal("cf-log-api-0"))
			})

			It("validate instance spec object", func() {
				m, err := dg.Manifest()
				Expect(err).ToNot(HaveOccurred())

				// Check JobInstance for the loggregator_trafficcontroller job
				jobInstancesRedis := m.InstanceGroups[1].Jobs[0].Properties.Quarks.Instances
				o := []JobInstance{
					{Address: "cf-log-api-0", AZ: "z1", Index: 0, Instance: 0, Name: "log-api-loggregator_trafficcontroller", Bootstrap: true, ID: "log-api-0-loggregator_trafficcontroller"},
					{Address: "cf-log-api-1", AZ: "z2", Index: 1, Instance: 0, Name: "log-api-loggregator_trafficcontroller", Bootstrap: false, ID: "log-api-1-loggregator_trafficcontroller"},
					{Address: "cf-log-api-2", AZ: "z1", Index: 2, Instance: 1, Name: "log-api-loggregator_trafficcontroller", Bootstrap: false, ID: "log-api-2-loggregator_trafficcontroller"},
					{Address: "cf-log-api-3", AZ: "z2", Index: 3, Instance: 1, Name: "log-api-loggregator_trafficcontroller", Bootstrap: false, ID: "log-api-3-loggregator_trafficcontroller"},
				}
				Expect(jobInstancesRedis).To(BeEquivalentTo(o))

			})

			Context("when manifest presets overridden bpm info", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithOverriddenBPMInfo()
					Expect(err).NotTo(HaveOccurred())
					ig = "redis-slave"
				})

				It("returns overwritten bpm config", func() {
					bpmConfigs, err := dg.BPMConfigs()
					Expect(err).ToNot(HaveOccurred())

					bpm := bpmConfigs["redis-server"]
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
					bpmConfigs, err := dg.BPMConfigs()
					Expect(err).ToNot(HaveOccurred())

					bpm := bpmConfigs["redis-server"]
					Expect(bpm).ToNot(BeNil())
					Expect(bpm.Processes[0].Executable).To(Equal("/var/vcap/packages/redis-4/bin/redis-server"))
					Expect(bpm.Processes[1].Name).To(Equal("absent-process"))
					Expect(bpm.Processes[1].Executable).To(Equal("/absent-process-command"))
				})
			})
		})

		Describe("Manifest", func() {
			BeforeEach(func() {
				m, err = env.ElaboratedBOSHManifest()
				Expect(err).NotTo(HaveOccurred())
				ig = "redis-slave"
			})

			It("should gather all data for each job spec file", func() {
				manifest, err := dg.Manifest()
				Expect(err).ToNot(HaveOccurred())

				//Check JobInstance for the redis-server job
				jobInstancesRedis := manifest.InstanceGroups[0].Jobs[0].Properties.Quarks.Instances

				compareToFakeRedis := []JobInstance{
					{Address: "foo-deployment-redis-slave-0", AZ: "z1", Index: 0, Instance: 0, Name: "redis-slave-redis-server", Bootstrap: true, ID: "redis-slave-0-redis-server"},
					{Address: "foo-deployment-redis-slave-1", AZ: "z2", Index: 1, Instance: 0, Name: "redis-slave-redis-server", Bootstrap: false, ID: "redis-slave-1-redis-server"},
					{Address: "foo-deployment-redis-slave-2", AZ: "z1", Index: 2, Instance: 1, Name: "redis-slave-redis-server", Bootstrap: false, ID: "redis-slave-2-redis-server"},
					{Address: "foo-deployment-redis-slave-3", AZ: "z2", Index: 3, Instance: 1, Name: "redis-slave-redis-server", Bootstrap: false, ID: "redis-slave-3-redis-server"},
				}
				Expect(jobInstancesRedis).To(BeEquivalentTo(compareToFakeRedis))
			})

			Context("when resolving links between providers and consumers", func() {
				BeforeEach(func() {
					m, err = env.BOSHManifestWithProviderAndConsumer()
					Expect(err).NotTo(HaveOccurred())
					ig = "log-api"
				})

				It("resolves all required data if the job consumes a link", func() {
					manifest, err := dg.Manifest()
					Expect(err).ToNot(HaveOccurred())

					// log-api instance_group, with loggregator_trafficcontroller job, consumes a link from doppler job
					jobQuarksConsumes := manifest.InstanceGroups[1].Jobs[0].Properties.Quarks.Consumes
					jobConsumesFromDoppler, consumeFromDopplerExists := jobQuarksConsumes["doppler"]
					Expect(consumeFromDopplerExists).To(BeTrue())
					expectedProperties := map[string]interface{}{
						"doppler": map[string]interface{}{
							"grpc_port": json.Number("7765"),
						},
						"fooprop": json.Number("10001"),
					}
					for i, instance := range jobConsumesFromDoppler.Instances {
						Expect(instance.Index).To(Equal(i))
						Expect(instance.Address).To(Equal(fmt.Sprintf("cf-doppler-%v", i)))
					}

					Expect(deep.Equal(jobConsumesFromDoppler.Properties, expectedProperties)).To(HaveLen(0))
				})

				It("has an empty consumes list if the job does not consume a link", func() {
					manifest, err := dg.Manifest()
					Expect(err).ToNot(HaveOccurred())

					// doppler instance_group, with doppler job, only provides doppler link
					jobQuarksConsumes := manifest.InstanceGroups[0].Jobs[0].Properties.Quarks.Consumes
					var emptyJobQuarksConsumes map[string]JobLink
					Expect(jobQuarksConsumes).To(BeEquivalentTo(emptyJobQuarksConsumes))
				})
			})
		})
	})
})
