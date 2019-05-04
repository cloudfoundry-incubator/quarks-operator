package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("variable-interpolation", func() {
	var (
		manifestPath string
		varsDir      string
	)

	act := func(manifestPath, varsDir string) (session *gexec.Session, err error) {
		args := []string{"util", "-m", manifestPath, "variable-interpolation", "-v", varsDir}
		cmd := exec.Command(cliPath, args...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Context("when manifest exists", func() {
		BeforeEach(func() {
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			manifestPath = filepath.Join(wd, "../testing/assets/manifest.yaml")
			varsDir = filepath.Join(wd, "../testing/assets/vars")
		})

		// TODO these three tests are meant to test different things?
		It("should show a interpolated manifest with variables files", func() {
			session, err := act(manifestPath, varsDir)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\\n"}`))
		})

		It("should show a json format", func() {
			session, err := act(manifestPath, varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\\n"}`))
		})

		It("should show a encode format", func() {
			session, err := act(manifestPath, varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\n"}`))
		})
	})
})
