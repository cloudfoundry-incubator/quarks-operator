package cmd_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "code.cloudfoundry.org/cf-operator/cmd/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Trender", func() {

	Context("When flag values and manifest file are specified", func() {

		var (
			deploymentManifest string
			jobsDir            string
			instanceGroupName  string
			index              int
		)

		BeforeEach(func() {
			deploymentManifest = "../../testing/assets/gatherManifest.yml"
			jobsDir = "../../testing/assets"
			instanceGroupName = "log-api"
		})

		Context("with an invalid instance index", func() {
			BeforeEach(func() {
				index = 1000
			})

			It("fails", func() {
				err := RenderData(deploymentManifest, jobsDir, instanceGroupName, index)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no instance found"))
			})
		})

		Context("with a valid instance index", func() {
			BeforeEach(func() {
				index = 0
			})

			It("renders the job erb files correctly", func() {
				err := RenderData(deploymentManifest, jobsDir, instanceGroupName, index)
				Expect(err).ToNot(HaveOccurred())

				absDestFile := filepath.Join(jobsDir, "loggregator_trafficcontroller", "config/bpm.yml")
				Expect(absDestFile).Should(BeAnExistingFile())

				// Unmarshal the rendered file
				bpmYmlBytes, err := ioutil.ReadFile(absDestFile)
				Expect(err).ToNot(HaveOccurred())

				var bpmYml map[string][]interface{}
				err = yaml.Unmarshal(bpmYmlBytes, &bpmYml)
				Expect(err).ToNot(HaveOccurred())

				// Check fields if they are rendered
				Expect(bpmYml["processes"][0].(map[interface{}]interface{})["env"].(map[interface{}]interface{})["AGENT_UDP_ADDRESS"]).To(Equal("127.0.0.1:3457"))
				Expect(bpmYml["processes"][0].(map[interface{}]interface{})["env"].(map[interface{}]interface{})["TRAFFIC_CONTROLLER_OUTGOING_DROPSONDE_PORT"]).To(Equal("8081"))
				Expect(bpmYml["processes"][0].(map[interface{}]interface{})["env"].(map[interface{}]interface{})["FOOBARWITHLINKINSTANCESAZ"]).To(Equal("z1"))

				// Delete  the file
				err = os.RemoveAll(filepath.Join(jobsDir, "loggregator_trafficcontroller"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
