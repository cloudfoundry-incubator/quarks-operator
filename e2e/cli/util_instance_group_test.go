package cli_test

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("instance-group", func() {
	var (
		manifestPath string
	)

	act := func(manifestPath string) (session *gexec.Session, err error) {
		args := []string{"util", "instance-group", "-m", manifestPath, "-b", assetPath, "-g", "log-api", "--output-file-path", assetPath + "/output.json"}
		cmd := exec.Command(cliPath, args...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Context("when manifest exists", func() {
		BeforeEach(func() {
			manifestPath = assetPath + "/gatherManifest.yml"
		})

		FIt("gathers data to stdout", func() {
			session, err := act(manifestPath)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			var output map[string]string
			dataBytes, err := ioutil.ReadFile(assetPath + "/output.json")
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(dataBytes, &output)
			Expect(err).ToNot(HaveOccurred())

			Expect(err).ToNot(HaveOccurred())
			Expect(string(dataBytes)).Should(ContainSubstring(`"properties.yaml":"`))
			Expect(string(dataBytes)).Should(ContainSubstring(`name: cf`))

			Expect(output["properties.yaml"]).ToNot(BeEmpty())
		})
	})
})
