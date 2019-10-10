package cli_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("variable-interpolation", func() {
	var (
		manifestPath string
		varsDir      string
	)

	act := func(manifestPath, varsDir string) (session *gexec.Session, err error) {
		args := []string{"util", "-m", manifestPath, "variable-interpolation", "-v", varsDir, "--output-file-path", filepath.Join(assetPath, "output.json")}
		cmd := exec.Command(cliPath, args...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Context("when manifest exists", func() {
		BeforeEach(func() {
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			manifestPath = filepath.Join(wd, assetPath+"/manifest.yaml")
			varsDir = filepath.Join(wd, assetPath+"/vars")
		})

		It("should show a encoded format", func() {
			session, err := act(manifestPath, varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			dataBytes, err := ioutil.ReadFile(filepath.Join(assetPath, "output.json"))
			Expect(err).ToNot(HaveOccurred())

			Expect(string(dataBytes)).To(Equal(`{"manifest.yaml":"director_uuid: \"\"\ninstance_groups:\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings: {}\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    baz\n  properties:\n    quarks: {}\n  stemcell: \"\"\n  vm_resources: null\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings: {}\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    foo\n  properties:\n    quarks: {}\n  stemcell: \"\"\n  vm_resources: null\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings: {}\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    bar\n  properties:\n    quarks: {}\n  stemcell: \"\"\n  vm_resources: null\nname: |\n  fake-password\n"}`))
		})
	})
})
