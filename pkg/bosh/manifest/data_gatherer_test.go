package manifest_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//. "github.com/onsi/gomega/gstruct"
	"go.uber.org/zap"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"
)

const assetPath = "../../../testing/assets"

var _ = Describe("DataGatherer", func() {

	var (
		m   *Manifest
		env testing.Catalog
		log *zap.SugaredLogger
		dg  *DataGatherer
		ig  string
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
				m = env.BOSHManifestWithProviderAndConsumer()
				job := m.InstanceGroups[0].Jobs[0]

				value, ok := job.Property("doppler.grpc_port")
				Expect(ok).To(BeTrue())
				Expect(value).To(BeEquivalentTo(7765))
			})
		})
	})

	Context("DataGatherer", func() {
		BeforeEach(func() {
			_, log = helper.NewTestLogger()
		})

		JustBeforeEach(func() {
			var err error
			dg, err = manifest.NewDataGatherer(log, assetPath, "default", m, ig)
			Expect(err).ToNot(HaveOccurred())
			err = dg.GatherData()
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("BPMConfig", func() {
			BeforeEach(func() {
				m = env.BOSHManifestWithProviderAndConsumer()
				ig = "log-api"
			})

			It("returns the bpm config for all jobs", func() {
				bpmConfigs, err := dg.BPMConfigs()
				Expect(err).ToNot(HaveOccurred())

				bpm := bpmConfigs["loggregator_trafficcontroller"]
				Expect(bpm).ToNot(BeNil())
				Expect(bpm.Processes[0].Executable).To(Equal("/var/vcap/packages/loggregator_trafficcontroller/trafficcontroller"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKVALUES"]).To(Equal("10001"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKNESTEDVALUES"]).To(Equal("7765"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKINSTANCESAZ"]).To(Equal("z1"))
				Expect(bpm.Processes[0].Env["FOOBARWITHLINKINSTANCESADDRESS"]).To(Equal("cf-doppler-0.default.svc.cluster.local"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECADDRESS"]).To(Equal("cf-log-api-0.default.svc.cluster.local"))
				Expect(bpm.Processes[0].Env["FOOBARWITHSPECDEPLOYMENT"]).To(Equal("cf"))
			})
		})

		Describe("EnrichedManifest", func() {
			BeforeEach(func() {
				m = env.ElaboratedBOSHManifest()
				ig = "redis-slave"
			})

			It("should gather all data for each job spec file", func() {
				manifest := dg.EnrichedManifest()

				//Check JobInstance for the redis-server job
				jobInstancesRedis := manifest.InstanceGroups[0].Jobs[0].Properties.BOSHContainerization.Instances

				compareToFakeRedis := []JobInstance{
					{Address: "foo-deployment-redis-slave-0.default.svc.cluster.local", AZ: "z1", ID: "redis-slave-0-redis-server", Index: 0, Instance: 0, Name: "redis-slave-redis-server"},
					{Address: "foo-deployment-redis-slave-1.default.svc.cluster.local", AZ: "z2", ID: "redis-slave-1-redis-server", Index: 1, Instance: 0, Name: "redis-slave-redis-server"},
					{Address: "foo-deployment-redis-slave-2.default.svc.cluster.local", AZ: "z1", ID: "redis-slave-2-redis-server", Index: 2, Instance: 1, Name: "redis-slave-redis-server"},
					{Address: "foo-deployment-redis-slave-3.default.svc.cluster.local", AZ: "z2", ID: "redis-slave-3-redis-server", Index: 3, Instance: 1, Name: "redis-slave-redis-server"},
				}
				Expect(jobInstancesRedis).To(BeEquivalentTo(compareToFakeRedis))
			})

			Context("when resolving links between providers and consumers", func() {
				BeforeEach(func() {
					m = env.BOSHManifestWithProviderAndConsumer()
					ig = "log-api"
				})

				It("resolves all required data if the job consumes a link", func() {
					manifest := dg.EnrichedManifest()

					// log-api instance_group, with loggregator_trafficcontroller job, consumes a link from doppler job
					jobBoshContainerizationConsumes := manifest.InstanceGroups[1].Jobs[0].Properties.BOSHContainerization.Consumes
					jobConsumesFromDoppler, consumeFromDopplerExists := jobBoshContainerizationConsumes["doppler"]
					Expect(consumeFromDopplerExists).To(BeTrue())
					expectedProperties := map[string]interface{}{
						"doppler": map[interface{}]interface{}{
							"grpc_port": 7765,
						},
						"fooprop": 10001,
					}
					for i, instance := range jobConsumesFromDoppler.Instances {
						Expect(instance.Index).To(Equal(i))
						Expect(instance.Address).To(Equal(fmt.Sprintf("cf-doppler-%v.default.svc.cluster.local", i)))
						Expect(instance.ID).To(Equal(fmt.Sprintf("doppler-%v-doppler", i)))
					}
					Expect(jobConsumesFromDoppler.Properties).To(BeEquivalentTo(expectedProperties))
				})

				It("has an empty consumes list if the job does not consume a link", func() {
					manifest := dg.EnrichedManifest()
					// doppler instance_group, with doppler job, only provides doppler link
					jobBoshContainerizationConsumes := manifest.InstanceGroups[0].Jobs[0].Properties.BOSHContainerization.Consumes
					var emptyJobBoshContainerizationConsumes map[string]JobLink
					Expect(jobBoshContainerizationConsumes).To(BeEquivalentTo(emptyJobBoshContainerizationConsumes))
				})
			})
		})
	})
})
