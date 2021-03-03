package manifest_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"

	"sigs.k8s.io/yaml"
)

var _ = Describe("RenderJobTemplates", func() {
	var (
		deploymentManifest string
		instanceGroupName  string
		azIndex            int
		index              int
		podIP              net.IP
		replicas           int
		// tests could run in parallel
		tmpDir string
	)

	act := func() error {
		return manifest.RenderJobTemplates(deploymentManifest, assetPath, tmpDir, instanceGroupName, podIP, azIndex, index, replicas, true)
	}

	readBPM := func(f string) bpm.Process {
		Expect(f).Should(BeAnExistingFile())

		bytes, err := ioutil.ReadFile(f)
		Expect(err).ToNot(HaveOccurred())

		var yml map[string][]bpm.Process
		err = yaml.Unmarshal(bytes, &yml)
		Expect(err).ToNot(HaveOccurred())
		Expect(yml["processes"]).To(HaveLen(1))

		return yml["processes"][0]
	}

	BeforeEach(func() {
		// Defaults, tests may vary
		replicas = 1
		azIndex = 0
		index = 0

		// tests use the real filesystem
		var err error
		tmpDir, err = ioutil.TempDir("", "template-render")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	When("podIP is nil", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.mysql-v1.yml"
			instanceGroupName = "mysql0"
			podIP = nil
		})

		It("fails", func() {
			err := act()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("the pod IP is empty"))
		})
	})

	When("flag values and manifest file are specified", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/gatherManifest.yml"
			instanceGroupName = "log-api"
			podIP = net.ParseIP("172.17.0.13")
			azIndex = 1
		})

		It("renders the job erb files correctly", func() {
			err := act()
			Expect(err).ToNot(HaveOccurred())

			By("Checking the content of the rendered file")

			// Check fields if they are rendered
			values := readBPM(filepath.Join(tmpDir, "loggregator_trafficcontroller", "config/bpm.yml"))
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
			Expect(values.Env["FOOBARWITHSPECID"]).To(Equal("log-api-z0-0"))
			Expect(values.Env["FOOBARWITHSPECINDEX"]).To(Equal("0"))
			Expect(values.Env["FOOBARWITHSPECNAME"]).To(Equal("log-api-loggregator_trafficcontroller"))
			Expect(values.Env["FOOBARWITHSPECNETWORKS"]).To(Equal(""))
			Expect(values.Env["FOOBARWITHSPECADDRESS"]).To(Equal("log-api-z0-0"))
			Expect(values.Env["FOOBARWITHSPECDEPLOYMENT"]).To(Equal(""))
			Expect(values.Env["FOOBARWITHSPECIP"]).To(Equal("172.17.0.13"))
		})
	})

	Context("with an empty instances array in consumes", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.mysql-v1.yml"
			instanceGroupName = "mysql0"
			podIP = net.ParseIP("172.17.0.13")
		})

		It("renders the job erb files correctly", func() {
			err := act()
			Expect(err).ToNot(HaveOccurred())

			drainFile := filepath.Join(tmpDir, "pxc-mysql", "bin/drain")
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

			// By using non-zero index , we can make sure that an evaluation of spec.bootstrap
			// will return false. See https://bosh.io/docs/jobs/#properties-spec for
			// more information.
			index = 10000
			podIP = net.ParseIP("172.17.0.13")
			azIndex = 1
		})

		It("renders the configuration erb file correctly", func() {
			err := act()
			Expect(err).ToNot(HaveOccurred())

			configFile := filepath.Join(tmpDir, "redis-server", "config/redis.conf")
			Expect(configFile).Should(BeAnExistingFile())

			content, err := ioutil.ReadFile(configFile)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("slaveof foo-deployment-redis-slave-0 637"))

		})
	})

	When("spec.id is used in a bosh release", func() {
		BeforeEach(func() {
			deploymentManifest = assetPath + "/ig-resolved.app-autoscaler.yml"
			instanceGroupName = "asmetrics"
			podIP = net.ParseIP("172.17.0.13")
		})

		It("usage of spec field in an ERB template should work", func() {
			err := act()
			Expect(err).ToNot(HaveOccurred())

			configFile := filepath.Join(tmpDir, "metricsserver", "config/metricsserver.yml")
			Expect(configFile).Should(BeAnExistingFile())

			content, err := ioutil.ReadFile(configFile)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("node_index: 0"))

		})
	})

	Context("job instance spec values", func() {
		type cases []struct {
			podOrdinal     int
			startupOrdinal int
			azIndex        int
			initial        bool
			beAddress      string
			beAZ           string
			beIndex        string
			beBootstrap    string
		}

		BeforeEach(func() {
			instanceGroupName = "log-api"
			podIP = net.ParseIP("172.17.0.13")
			replicas = 2
		})

		When("multiple azs are used", func() {
			BeforeEach(func() {
				deploymentManifest = assetPath + "/templateRenderManifest.yml"
			})

			It("renders the job erb files correctly", func() {
				tests := cases{
					// two az
					{0, 0, 1, true, "log-api-z0-0", "z1", "0", "true"},
					{1, 1, 1, true, "log-api-z0-1", "z1", "1", "false"},
					{0, 0, 2, true, "log-api-z1-0", "z2", "2", "false"},
					{1, 1, 2, true, "log-api-z1-1", "z2", "3", "false"},

					// // two az, updated
					{0, 1, 1, false, "log-api-z0-0", "z1", "0", "false"},
					{1, 0, 1, false, "log-api-z0-1", "z1", "1", "false"}, // TODO would have expected this to be bootstrap
					{0, 1, 2, false, "log-api-z1-0", "z2", "2", "false"},
					{1, 0, 2, false, "log-api-z1-1", "z2", "3", "true"},

					// TODO two az, happily generates out of bounds - nothing we can do, replicas is automatically increased
					// {20, 20, 1, true, "log-api-z0-20", "z1", "20", "false"},
					// FIXME this will fail because of invalid AZ
					// {20, 20, 3, true, "log-api-z1-20", "z2", "5", "false"},

				}

				for i, t := range tests {
					err := manifest.RenderJobTemplates(deploymentManifest, assetPath, tmpDir, instanceGroupName, podIP, t.azIndex, t.podOrdinal, replicas, t.initial)
					Expect(err).ToNot(HaveOccurred())
					values := readBPM(filepath.Join(tmpDir, "loggregator_trafficcontroller", "config/spec.yml"))
					errstr := fmt.Sprintf("test case %d", i+1)
					Expect(values.Env["SPEC_ADDRESS"]).To(Equal(t.beAddress), errstr)
					Expect(values.Env["SPEC_AZ"]).To(Equal(t.beAZ), errstr)
					Expect(values.Env["SPEC_INDEX"]).To(Equal(t.beIndex), errstr)
					Expect(values.Env["SPEC_BOOTSTRAP"]).To(Equal(t.beBootstrap), errstr)
				}
			})
		})

		When("one azs is used", func() {
			BeforeEach(func() {
				deploymentManifest = assetPath + "/templateRenderManifestOneAZ.yml"
			})

			It("renders the job erb files correctly", func() {
				tests := cases{
					// single az
					{0, 0, 1, true, "log-api-z0-0", "z1", "0", "true"},
					{1, 1, 1, true, "log-api-z0-1", "z1", "1", "false"},

					// single az, updated
					{0, 1, 1, false, "log-api-z0-0", "z1", "0", "false"},
					{1, 0, 1, false, "log-api-z0-1", "z1", "1", "true"},
				}

				for i, t := range tests {
					err := manifest.RenderJobTemplates(deploymentManifest, assetPath, tmpDir, instanceGroupName, podIP, t.azIndex, t.podOrdinal, replicas, t.initial)
					Expect(err).ToNot(HaveOccurred())
					values := readBPM(filepath.Join(tmpDir, "loggregator_trafficcontroller", "config/spec.yml"))
					errstr := fmt.Sprintf("test case %d", i+1)
					Expect(values.Env["SPEC_ADDRESS"]).To(Equal(t.beAddress), errstr)
					Expect(values.Env["SPEC_AZ"]).To(Equal(t.beAZ), errstr)
					Expect(values.Env["SPEC_INDEX"]).To(Equal(t.beIndex), errstr)
					Expect(values.Env["SPEC_BOOTSTRAP"]).To(Equal(t.beBootstrap), errstr)
				}
			})
		})

		When("no azs are used", func() {
			BeforeEach(func() {
				deploymentManifest = assetPath + "/templateRenderManifestNoAZs.yml"
			})

			It("renders the job erb files correctly", func() {
				tests := cases{
					// no az
					{0, 0, 0, true, "log-api-0", "", "0", "true"},
					{1, 1, 0, true, "log-api-1", "", "1", "false"},

					// no az, updated
					{0, 1, 0, false, "log-api-0", "", "0", "false"},
					{1, 0, 0, false, "log-api-1", "", "1", "true"},
				}

				for i, t := range tests {
					err := manifest.RenderJobTemplates(deploymentManifest, assetPath, tmpDir, instanceGroupName, podIP, t.azIndex, t.podOrdinal, replicas, t.initial)
					Expect(err).ToNot(HaveOccurred())
					values := readBPM(filepath.Join(tmpDir, "loggregator_trafficcontroller", "config/spec.yml"))
					errstr := fmt.Sprintf("test case %d", i+1)
					Expect(values.Env["SPEC_ADDRESS"]).To(Equal(t.beAddress), errstr)
					Expect(values.Env["SPEC_AZ"]).To(Equal(t.beAZ), errstr)
					Expect(values.Env["SPEC_INDEX"]).To(Equal(t.beIndex), errstr)
					Expect(values.Env["SPEC_BOOTSTRAP"]).To(Equal(t.beBootstrap), errstr)
				}
			})
		})
	})
})
