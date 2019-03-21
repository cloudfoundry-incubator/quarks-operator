package cmd_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/cmd/internal"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Dgather", func() {

	var (
		m   *manifest.Manifest
		env testing.Catalog
	)

	Context("helper functions to override job specs from manifest", func() {
		It("should find a property value in the manifest job properties section (constructed example)", func() {
			// health.disk.warning
			exampleJob := manifest.Job{
				Properties: map[string]interface{}{
					"health": map[interface{}]interface{}{
						"disk": map[interface{}]interface{}{
							"warning": 42,
						},
					},
				},
			}

			var (
				value interface{}
				ok    bool
			)

			value, ok = LookUpProperty(exampleJob, "health.disk.warning")
			Expect(ok).To(BeTrue())
			Expect(value).To(BeEquivalentTo(42))

			value, ok = LookUpProperty(exampleJob, "health.disk.nonexisting")
			Expect(ok).To(BeFalse())
			Expect(value).To(BeNil())
		})

		It("should find a property value in the manifest job properties section (proper manifest example)", func() {
			m := env.BOSHManifestWithProviderAndConsumer()
			job := m.InstanceGroups[0].Jobs[0]

			var (
				value interface{}
				ok    bool
			)

			value, ok = LookUpProperty(job, "doppler.grpc_port")
			Expect(ok).To(BeTrue())
			Expect(value).To(BeEquivalentTo(7765))
		})
	})

	Context("gather job release specs and generate provider links", func() {
		BeforeEach(func() {
			m = env.ElaboratedBOSHManifest()
		})

		It("should gather all data for each job spec file", func() {
			releaseSpecs, _, err := CollectReleaseSpecsAndProviderLinks(m, "../../testing/assets/", "default", []string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(releaseSpecs)).To(Equal(2))

			//Check releaseSpecs for the redis job.MF test file
			redisReleaseSpec := releaseSpecs["redis"]["redis-server"]
			Expect(len(redisReleaseSpec.Templates)).To(Equal(4))
			Expect(len(redisReleaseSpec.Properties)).To(Equal(12))
			Expect(redisReleaseSpec.Consumes[0]).To(MatchFields(IgnoreMissing, Fields{
				"Name":     Equal("redis"),
				"Type":     Equal("redis"),
				"Optional": Equal(true),
			}))
			Expect(redisReleaseSpec.Provides[0]).To(MatchFields(IgnoreExtras, Fields{
				"Name":       Equal("redis"),
				"Type":       Equal("redis"),
				"Properties": ConsistOf("port", "password", "base_dir"),
			}))

			//Check releaseSpecs for the cflinuxfs3 job.MF test file
			cfLinuxReleaseSpec := releaseSpecs["cflinuxfs3"]["cflinuxfs3-rootfs-setup"]
			Expect(len(cfLinuxReleaseSpec.Templates)).To(Equal(2))
			Expect(len(cfLinuxReleaseSpec.Properties)).To(Equal(1))
			Expect(len(cfLinuxReleaseSpec.Consumes)).To(Equal(0))
			Expect(len(cfLinuxReleaseSpec.Provides)).To(Equal(0))
		})
		It("should have properties/bosh_containerization/instances populated for each job", func() {
			_, _, err := CollectReleaseSpecsAndProviderLinks(m, "../../testing/assets/", "default", []string{})
			Expect(err).ToNot(HaveOccurred())

			//Check JobInstance for the redis-server job
			jobInstancesRedis := m.InstanceGroups[0].Jobs[0].Properties["bosh_containerization"].(map[string]interface{})["instances"]
			compareToFakeRedis := []manifest.JobInstance{
				{Address: "redis-slave-0-redis-server.default.svc.cluster.local", AZ: "z1", ID: "redis-slave-0-redis-server", Index: 0, Instance: 0, Name: "redis-slave-redis-server"},
				{Address: "redis-slave-1-redis-server.default.svc.cluster.local", AZ: "z2", ID: "redis-slave-1-redis-server", Index: 1, Instance: 0, Name: "redis-slave-redis-server"},
				{Address: "redis-slave-2-redis-server.default.svc.cluster.local", AZ: "z1", ID: "redis-slave-2-redis-server", Index: 2, Instance: 1, Name: "redis-slave-redis-server"},
				{Address: "redis-slave-3-redis-server.default.svc.cluster.local", AZ: "z2", ID: "redis-slave-3-redis-server", Index: 3, Instance: 1, Name: "redis-slave-redis-server"},
			}
			Expect(jobInstancesRedis).To(BeEquivalentTo(compareToFakeRedis))

			//Check JobInstance for the cflinuxfs3-rootfs-setup job
			jobInstancesCell := m.InstanceGroups[1].Jobs[0].Properties["bosh_containerization"].(map[string]interface{})["instances"]
			compareToFakeCell := []manifest.JobInstance{
				{Address: "diego-cell-0-cflinuxfs3-rootfs-setup.default.svc.cluster.local", AZ: "z1", ID: "diego-cell-0-cflinuxfs3-rootfs-setup", Index: 0, Instance: 0, Name: "diego-cell-cflinuxfs3-rootfs-setup"},
				{Address: "diego-cell-1-cflinuxfs3-rootfs-setup.default.svc.cluster.local", AZ: "z2", ID: "diego-cell-1-cflinuxfs3-rootfs-setup", Index: 1, Instance: 0, Name: "diego-cell-cflinuxfs3-rootfs-setup"},
				{Address: "diego-cell-2-cflinuxfs3-rootfs-setup.default.svc.cluster.local", AZ: "z1", ID: "diego-cell-2-cflinuxfs3-rootfs-setup", Index: 2, Instance: 1, Name: "diego-cell-cflinuxfs3-rootfs-setup"},
				{Address: "diego-cell-3-cflinuxfs3-rootfs-setup.default.svc.cluster.local", AZ: "z2", ID: "diego-cell-3-cflinuxfs3-rootfs-setup", Index: 3, Instance: 1, Name: "diego-cell-cflinuxfs3-rootfs-setup"},
			}
			Expect(jobInstancesCell).To(BeEquivalentTo(compareToFakeCell))
		})

		It("should get all links from providers", func() {
			_, providerLinks, err := CollectReleaseSpecsAndProviderLinks(m, "../../testing/assets/", "default", []string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(providerLinks)).To(BeEquivalentTo(1))
			expectedInstances := []manifest.JobInstance{
				{Address: "redis-slave-0-redis-server.default.svc.cluster.local", AZ: "z1", ID: "redis-slave-0-redis-server", Index: 0, Instance: 0, Name: "redis-slave-redis-server"},
				{Address: "redis-slave-1-redis-server.default.svc.cluster.local", AZ: "z2", ID: "redis-slave-1-redis-server", Index: 1, Instance: 0, Name: "redis-slave-redis-server"},
				{Address: "redis-slave-2-redis-server.default.svc.cluster.local", AZ: "z1", ID: "redis-slave-2-redis-server", Index: 2, Instance: 1, Name: "redis-slave-redis-server"},
				{Address: "redis-slave-3-redis-server.default.svc.cluster.local", AZ: "z2", ID: "redis-slave-3-redis-server", Index: 3, Instance: 1, Name: "redis-slave-redis-server"},
			}
			expectedProperties := map[string]interface{}{
				"port":     6379,
				"password": "foobar",
				"base_dir": "/var/vcap/store/redis",
			}
			//Check that Instances in the link are correct
			Expect(providerLinks["redis"]["redis-server"].Instances).To(BeEquivalentTo(expectedInstances))
			Expect(providerLinks["redis"]["redis-server"].Properties).To(BeEquivalentTo(expectedProperties))
		})
	})

	Context("resolve links between providers and consumers", func() {
		BeforeEach(func() {
			m = env.BOSHManifestWithProviderAndConsumer()
		})

		It("should get all required data if the job consumes a link", func() {
			releaseSpecs, links, _ := CollectReleaseSpecsAndProviderLinks(m, "../../testing/assets/", "default", []string{})
			err := GetConsumersAndRenderERB(m, "../../testing/assets/", releaseSpecs, links)

			// log-api instance_group, with loggregator_trafficcontroller job, consumes a link from
			// doppler job
			jobBoshContainerizationProperties := m.InstanceGroups[1].Jobs[0].Properties["bosh_containerization"]
			jobBoshContainerizationConsumes, consumeIsNotEmpty := jobBoshContainerizationProperties.(map[string]interface{})["consumes"]

			Expect(len(releaseSpecs)).To(Equal(1)) // only one release per manifest.yml sample
			Expect(err).ToNot(HaveOccurred())
			Expect(consumeIsNotEmpty).To(BeTrue())

			jobConsumesFromDoppler, consumeFromDopplerExists := jobBoshContainerizationConsumes.(map[string]manifest.JobLink)["doppler"]
			expectedProperties := map[string]interface{}{
				"doppler": map[string]interface{}{
					"grpc_port": 7765,
				},
				"fooprop": 10001,
			}

			Expect(consumeFromDopplerExists).To(BeTrue())
			for i, instance := range jobConsumesFromDoppler.Instances {
				Expect(instance.Index).To(Equal(i))
				Expect(instance.Address).To(Equal(fmt.Sprintf("doppler-%v-doppler.default.svc.cluster.local", i)))
				Expect(instance.ID).To(Equal(fmt.Sprintf("doppler-%v-doppler", i)))
			}
			Expect(jobConsumesFromDoppler.Properties).To(BeEquivalentTo(expectedProperties))
		})

		It("should get nothing if the job does not consumes a link", func() {
			releaseSpecs, links, _ := CollectReleaseSpecsAndProviderLinks(m, "../../testing/assets/", "default", []string{})
			err := GetConsumersAndRenderERB(m, "../../testing/assets/", releaseSpecs, links)

			// doppler instance_group, with doppler job, only provides doppler link
			jobBoshContainerizationProperties := m.InstanceGroups[0].Jobs[0].Properties["bosh_containerization"]
			jobBoshContainerizationConsumes, _ := jobBoshContainerizationProperties.(map[string]interface{})["consumes"]
			emptyJobBoshContainerizationConsumes := map[string]manifest.JobLink{}

			Expect(err).ToNot(HaveOccurred())
			Expect(jobBoshContainerizationConsumes).To(BeEquivalentTo(emptyJobBoshContainerizationConsumes))
		})
	})

	Context("rendering ERB files", func() {
		BeforeEach(func() {
			m = env.BOSHManifestWithProviderAndConsumer()
		})

		It("should render complex ERB files", func() {
			releaseSpecs, links, err := CollectReleaseSpecsAndProviderLinks(m, "../../testing/assets/", "default", []string{})
			anotherErr := GetConsumersAndRenderERB(m, "../../testing/assets/", releaseSpecs, links)
			Expect(err).ToNot(HaveOccurred())
			Expect(anotherErr).ToNot(HaveOccurred())

			jobBoshContainerizationProperties := m.InstanceGroups[1].Jobs[0].Properties["bosh_containerization"]
			jobBoshContainerizationPropertiesInstances := jobBoshContainerizationProperties.(map[string]interface{})["instances"]
			Expect(len(jobBoshContainerizationPropertiesInstances.([]manifest.JobInstance))).To(Equal(4))

			propertiesInstance := jobBoshContainerizationPropertiesInstances.([]manifest.JobInstance)[0]

			// in ERB file, there are test environment variables like these:
			//   FOOBARWITHLINKVALUES: <%= link('doppler').p("fooprop") %>
			//   FOOBARWITHLINKNESTEDVALUES: <%= link('doppler').p("doppler.grpc_port") %>
			//   FOOBARWITHLINKINSTANCESINDEX: <%= link('doppler').instances[0].index %>
			//   FOOBARWITHLINKINSTANCESAZ: <%= link('doppler').instances[0].az %>
			//   FOOBARWITHLINKINSTANCESADDRESS: <%= link('doppler').instances[0].address %>
			//   ...

			// For the first instance
			bpmProcesses := propertiesInstance.BPM.(*bpm.Config).Processes[0]
			Expect(bpmProcesses.Env["FOOBARWITHLINKVALUES"]).To(Equal("10001"))
			Expect(bpmProcesses.Env["FOOBARWITHLINKNESTEDVALUES"]).To(Equal("7765"))

			Expect(bpmProcesses.Env["FOOBARWITHLINKINSTANCESAZ"]).To(Equal("z1"))
			Expect(bpmProcesses.Env["FOOBARWITHLINKINSTANCESADDRESS"]).To(Equal("doppler-0-doppler.default.svc.cluster.local"))
			Expect(bpmProcesses.Env["FOOBARWITHSPECADDRESS"]).To(Equal("log-api-0-loggregator_trafficcontroller.default.svc.cluster.local"))
			Expect(bpmProcesses.Env["FOOBARWITHSPECDEPLOYMENT"]).To(Equal("cf"))

			// For the second instance
			propertiesInstance = jobBoshContainerizationPropertiesInstances.([]manifest.JobInstance)[1]
			bpmProcesses = propertiesInstance.BPM.(*bpm.Config).Processes[0]
			Expect(bpmProcesses.Env["FOOBARWITHSPECADDRESS"]).To(Equal("log-api-1-loggregator_trafficcontroller.default.svc.cluster.local"))

			// For the third instance
			propertiesInstance = jobBoshContainerizationPropertiesInstances.([]manifest.JobInstance)[2]
			bpmProcesses = propertiesInstance.BPM.(*bpm.Config).Processes[0]
			Expect(bpmProcesses.Env["FOOBARWITHSPECADDRESS"]).To(Equal("log-api-2-loggregator_trafficcontroller.default.svc.cluster.local"))

			// For the fourth instance
			propertiesInstance = jobBoshContainerizationPropertiesInstances.([]manifest.JobInstance)[3]
			Expect(propertiesInstance.BPM).To(BeNil())
		})
	})
})
