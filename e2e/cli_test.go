package e2e_test

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("CLI", func() {
	act := func(arg ...string) (session *gexec.Session, err error) {
		cmd := exec.Command(cliPath, arg...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Describe("help", func() {
		It("should show the help for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Usage:`))
		})

		It("should show all available options for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
  -h, --help                help for cf-operator
  -c, --kubeconfig string   Path to a kubeconfig, not required in-cluster
  -m, --master string       Kubernetes API server address
  -n, --namespace string    Namespace to watch for BOSH deployments \(default "default"\)

`))
		})

		It("shows all available commands", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Available Commands:
  help        Help about any command
  version     Print the version number

`))
		})
	})

	Describe("default", func() {
		It("should start the server", func() {
			session, err := act()
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say(`Starting cf-operator with namespace default`))
		})

		Context("when using environment variables", func() {
			BeforeEach(func() {
				os.Setenv("CFO_NAMESPACE", "env-test")
			})

			AfterEach(func() {
				os.Setenv("CFO_NAMESPACE", "")
			})

			It("should start for namespace", func() {
				session, err := act()
				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Err).Should(Say(`Starting cf-operator with namespace env-test`))
			})
		})

		Context("when using switches", func() {
			It("should start for namespace", func() {
				session, err := act("--namespace", "switch-test")
				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Err).Should(Say(`Starting cf-operator with namespace switch-test`))
			})
		})
	})

	Describe("version", func() {
		It("should show a semantic version number", func() {
			session, err := act("version")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`CF-Operator Version: \d+.\d+.\d+`))
		})
	})
})
