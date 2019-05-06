package e2e_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("render-template", func() {
	var (
		manifestPath string
		tmpDir       string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "ginkgo-run")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).ShouldNot(HaveOccurred())
	})

	act := func(manifestPath string) (session *gexec.Session, err error) {
		args := []string{"util", "template-render", "-m", manifestPath, "-j", "../testing/assets", "-g", "log-api", "--spec-index", "0", "-d", tmpDir}
		cmd := exec.Command(cliPath, args...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Context("when manifest exists", func() {
		BeforeEach(func() {
			manifestPath = "../testing/assets/gatherManifest.yml"
		})

		It("rendert the instance group template to a file", func() {
			session, err := act(manifestPath)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			absDestFile := filepath.Join(tmpDir, "loggregator_trafficcontroller", "config/bpm.yml")
			Expect(absDestFile).Should(BeAnExistingFile())
		})
	})
})
