package cli_test

import (
	"encoding/json"
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
		args := []string{"util", "instance-group", "-m", manifestPath, "-b", assetPath, "-g", "log-api"}
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
			output := session.Out.Contents()
			Expect(output).Should(ContainSubstring(`"properties.yaml":"`))
			Expect(output).Should(ContainSubstring(`name: cf`))

			var yml map[string]interface{}
			err = json.Unmarshal(output, &yml)
			Expect(err).ToNot(HaveOccurred())

			Expect(yml["properties.yaml"]).ToNot(BeEmpty())
		})
	})
})
