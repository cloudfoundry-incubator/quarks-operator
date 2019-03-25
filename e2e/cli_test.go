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
  -o, --docker-image-org string          Dockerhub organization that provides the operator docker image \(default "cfcontainerization"\)
  -r, --docker-image-repository string   Dockerhub repository that provides the operator docker image \(default "cf-operator"\)
  -t, --docker-image-tag string          Tag of the operator docker image \(default "\d+.\d+.\d+"\)
  -h, --help                             help for cf-operator
  -c, --kubeconfig string                Path to a kubeconfig, not required in-cluster
  -n, --namespace string                 Namespace to watch for BOSH deployments \(default "default"\)
  -w, --operator-webhook-host string     Hostname/IP under which the webhook server can be reached from the cluster
  -p, --operator-webhook-port string     Port the webhook server listens on \(default "2999"\)`))
		})

		It("shows all available commands", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Available Commands:
  data-gather            Gathers data of a bosh manifest
  help                   Help about any command
  template-render        Renders a bosh manifest
  variable-interpolation Interpolate variables
  version                Print the version number

`))
		})
	})

	Describe("default", func() {
		It("should start the server", func() {
			session, err := act()
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace`))
		})

		Context("when specifying namespace", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("CFO_NAMESPACE", "env-test")
				})

				AfterEach(func() {
					os.Setenv("CFO_NAMESPACE", "")
				})

				It("should start for namespace", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace env-test`))
				})
			})

			Context("via using switches", func() {
				It("should start for namespace", func() {
					session, err := act("--namespace", "switch-test")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace switch-test`))
				})
			})
		})

		Context("when specifying kubeconfig", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("KUBECONFIG", "invalid")
				})

				AfterEach(func() {
					os.Setenv("KUBECONFIG", "")
				})

				It("should use specified config", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`stat invalid: no such file or directory`))
				})
			})

			Context("via switches", func() {
				It("should use specified config", func() {
					session, err := act("--kubeconfig", "invalid")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`stat invalid: no such file or directory`))
				})
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

	Describe("variable-interpolation", func() {
		It("should show a interpolated manifest with variables files", func() {
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			manifestPath := filepath.Join(wd, "../testing/assets/manifest.yaml")
			varsDir := filepath.Join(wd, "../testing/assets/vars")

			session, err := act("variable-interpolation", "-m", manifestPath, "-v", varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`instance-group:
  key1: baz
  key2: foo
  key3: bar
password: fake-password`))
		})
	})
})
