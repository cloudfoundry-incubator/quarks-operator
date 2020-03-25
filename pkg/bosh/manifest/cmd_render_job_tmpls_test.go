package manifest_test

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"

	"sigs.k8s.io/yaml"
)

var _ = Describe("Trender", func() {
	var (
		deploymentManifest string
		instanceGroupName  string
		index              int
		podIP              net.IP
		replicas           int
	)

	const jobsDir = "../../../testing/assets"

	Context("when podIP is nil", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.mysql-v1.yml"
			instanceGroupName = "mysql0"
			index = 0
			podIP = nil
			replicas = 1
		})

		act := func() error {
			return manifest.RenderJobTemplates(deploymentManifest, jobsDir, jobsDir, instanceGroupName, index, podIP, replicas, true)
		}

		It("fails", func() {
			err := act()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("the pod IP is empty"))
		})
	})

	Context("when flag values and manifest file are specified", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/gatherManifest.yml"
			instanceGroupName = "log-api"
			podIP = net.ParseIP("172.17.0.13")
			replicas = 1
		})

		act := func() error {
			return manifest.RenderJobTemplates(deploymentManifest, jobsDir, jobsDir, instanceGroupName, index, podIP, replicas, true)
		}

		Context("with an invalid instance index", func() {
			BeforeEach(func() {
				index = 1000
			})

			It("fails", func() {
				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no job instance found"))
			})
		})

		Context("with a valid instance index", func() {
			BeforeEach(func() {
				index = 0
			})

			It("renders the job erb files correctly", func() {
				err := act()
				Expect(err).ToNot(HaveOccurred())

				absDestFile := filepath.Join(jobsDir, "loggregator_trafficcontroller", "config/bpm.yml")
				Expect(absDestFile).Should(BeAnExistingFile())

				By("Checking the content of the rendered file")
				bpmYmlBytes, err := ioutil.ReadFile(absDestFile)
				Expect(err).ToNot(HaveOccurred())

				var bpmYml map[string][]bpm.Process
				err = yaml.Unmarshal(bpmYmlBytes, &bpmYml)
				Expect(err).ToNot(HaveOccurred())

				// Check fields if they are rendered
				values := bpmYml["processes"][0]
				Expect(values.Env["AGENT_UDP_ADDRESS"]).To(Equal("127.0.0.1:3457"))
				Expect(values.Env["TRAFFIC_CONTROLLER_OUTGOING_DROPSONDE_PORT"]).To(Equal("8081"))
				Expect(values.Env["FOOBARWITHLINKINSTANCESAZ"]).To(Equal("z1"))

				// The following block of assertions are related to the usage of
				// the BPM spec object instance in an ERB test file, see
				// https://bosh.io/docs/jobs/#properties-spec
				//
				// When rendering a BPM ERB file for the release templates,the values for an spec object,
				// will use the instance at the index provided to the RenderJobTemplates func().
				Expect(values.Env["FOOBARWITHSPECAZ"]).To(Equal("z1"))
				Expect(values.Env["FOOBARWITHSPECBOOTSTRAP"]).To(Equal("true"))
				Expect(values.Env["FOOBARWITHSPECID"]).To(Equal("log-api-0"))
				Expect(values.Env["FOOBARWITHSPECINDEX"]).To(Equal("0"))
				Expect(values.Env["FOOBARWITHSPECNAME"]).To(Equal("log-api-loggregator_trafficcontroller"))
				Expect(values.Env["FOOBARWITHSPECNETWORKS"]).To(Equal(""))
				Expect(values.Env["FOOBARWITHSPECADDRESS"]).To(Equal("log-api-z0-0"))
				Expect(values.Env["FOOBARWITHSPECDEPLOYMENT"]).To(Equal(""))
				Expect(values.Env["FOOBARWITHSPECIP"]).To(Equal("172.17.0.13"))
			})

			AfterEach(func() {
				err := os.RemoveAll(filepath.Join(jobsDir, "loggregator_trafficcontroller"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("with an empty instances array in consumes", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.mysql-v1.yml"
			instanceGroupName = "mysql0"
			index = 0
			podIP = net.ParseIP("172.17.0.13")
			replicas = 1
		})

		AfterEach(func() {
			err := os.RemoveAll(filepath.Join(jobsDir, "pxc-mysql"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("renders the job erb files correctly", func() {
			err := manifest.RenderJobTemplates(deploymentManifest, jobsDir, jobsDir, instanceGroupName, index, podIP, replicas, true)
			Expect(err).ToNot(HaveOccurred())

			drainFile := filepath.Join(jobsDir, "pxc-mysql", "bin/drain")
			Expect(drainFile).Should(BeAnExistingFile())

			content, err := ioutil.ReadFile(drainFile)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("#!/usr/bin/env bash"))
			Expect(string(content)).To(ContainSubstring("foo=8335xx"))
			Expect(string(content)).To(ContainSubstring("bar=123987xx"))
			Expect(string(content)).To(ContainSubstring("maximum_size: 999999size"))
			Expect(string(content)).To(ContainSubstring("maximum_size2: 1000000size"))
			Expect(string(content)).To(ContainSubstring("maximum_size3: 1000001size"))
		})
	})

	Context("with an instance index if one", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.redis.yml"
			instanceGroupName = "redis-slave"

			// By using index 1, we can make sure that an evaluation of spec.bootstrap
			// will return false. See https://bosh.io/docs/jobs/#properties-spec for
			// more information.
			index = 1
			podIP = net.ParseIP("172.17.0.13")
			replicas = 1
		})

		AfterEach(func() {
			err := os.RemoveAll(filepath.Join(jobsDir, "redis-server"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("renders the configuration erb file correctly", func() {
			err := manifest.RenderJobTemplates(deploymentManifest, jobsDir, jobsDir, instanceGroupName, index, podIP, replicas, true)
			Expect(err).ToNot(HaveOccurred())

			configFile := filepath.Join(jobsDir, "redis-server", "config/redis.conf")
			Expect(configFile).Should(BeAnExistingFile())

			content, err := ioutil.ReadFile(configFile)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("slaveof foo-deployment-redis-slave-0 637"))

		})
	})

	Context("when spec.id is used in a bosh release", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.app-autoscaler.yml"
			instanceGroupName = "asmetrics"
			index = 0
			podIP = net.ParseIP("172.17.0.13")
			replicas = 1
		})

		AfterEach(func() {
			err := os.RemoveAll(filepath.Join(jobsDir, "metricsserver"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("usage of spec field in an ERB template should work", func() {
			err := manifest.RenderJobTemplates(deploymentManifest, jobsDir, jobsDir, instanceGroupName, index, podIP, replicas, true)
			Expect(err).ToNot(HaveOccurred())

			configFile := filepath.Join(jobsDir, "metricsserver", "config/metricsserver.yml")
			Expect(configFile).Should(BeAnExistingFile())

			content, err := ioutil.ReadFile(configFile)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("node_index: 0"))

		})
	})
})
