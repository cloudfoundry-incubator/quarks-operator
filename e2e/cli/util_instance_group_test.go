package cli_test

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"sigs.k8s.io/yaml"
)

var _ = Describe("instance-group", func() {
	var (
		manifestPath string
	)

	act := func(manifestPath string) (session *gexec.Session, err error) {
		args := []string{"util", "instance-group", "-n", "foo", "-m", manifestPath, "-b", assetPath, "-g", "log-api", "--output-file-path", assetPath}
		cmd := exec.Command(cliPath, args...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Context("when manifest exists", func() {
		BeforeEach(func() {
			manifestPath = assetPath + "/gatherManifest.yml"
		})

		It("gathers data to stdout", func() {
			session, err := act(manifestPath)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			var output map[string]string
			dataBytes, err := ioutil.ReadFile(assetPath + "/ig.json")
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(dataBytes, &output)
			Expect(err).ToNot(HaveOccurred())

			Expect(err).ToNot(HaveOccurred())
			Expect(string(dataBytes)).Should(ContainSubstring(`"properties.yaml":"`))
			Expect(string(dataBytes)).Should(ContainSubstring(`instance_groups:`))

			Expect(output["properties.yaml"]).ToNot(BeEmpty())

			dataBytes, err = ioutil.ReadFile(filepath.Join(assetPath, "bpm.json"))
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(dataBytes, &output)
			Expect(err).ToNot(HaveOccurred())

			bpmInfo := manifest.BPMInfo{}
			err = yaml.Unmarshal([]byte(output["bpm.yaml"]), &bpmInfo)
			Expect(err).ToNot(HaveOccurred())

			config := bpmInfo.Configs["loggregator_trafficcontroller"]
			Expect(len(config.Processes)).To(Equal(1))
			Expect(config.Processes[0].Executable).To(Equal("/var/vcap/packages/loggregator_trafficcontroller/trafficcontroller"))
		})
	})
})
