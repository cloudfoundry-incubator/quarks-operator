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
  -n, --cf-operator-namespace string     \(CF_OPERATOR_NAMESPACE\) Namespace to watch for BOSH deployments \(default "default"\)
  -o, --docker-image-org string          \(DOCKER_IMAGE_ORG\) Dockerhub organization that provides the operator docker image \(default "cfcontainerization"\)
  -r, --docker-image-repository string   \(DOCKER_IMAGE_REPOSITORY\) Dockerhub repository that provides the operator docker image \(default "cf-operator"\)
  -t, --docker-image-tag string          \(DOCKER_IMAGE_TAG\) Tag of the operator docker image \(default "\d+.\d+.\d+"\)
  -h, --help                             help for cf-operator
  -c, --kubeconfig string                \(KUBECONFIG\) Path to a kubeconfig, not required in-cluster
  -w, --operator-webhook-host string     \(CF_OPERATOR_WEBHOOK_HOST\) Hostname/IP under which the webhook server can be reached from the cluster
  -p, --operator-webhook-port string     \(CF_OPERATOR_WEBHOOK_PORT\) Port the webhook server listens on \(default "2999"\)`))
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
					os.Setenv("CF_OPERATOR_NAMESPACE", "env-test")
				})

				AfterEach(func() {
					os.Setenv("CF_OPERATOR_NAMESPACE", "")
				})

				It("should start for namespace", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace env-test`))
				})
			})

			Context("via using switches", func() {
				It("should start for namespace", func() {
					session, err := act("--cf-operator-namespace", "switch-test")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace switch-test`))
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

			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\\n"}`))
		})

		It("should show a json format", func() {
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			manifestPath := filepath.Join(wd, "../testing/assets/manifest.yaml")
			varsDir := filepath.Join(wd, "../testing/assets/vars")

			session, err := act("variable-interpolation", "-m", manifestPath, "-v", varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\\n"}`))
		})

		It("should show a encode format", func() {
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			manifestPath := filepath.Join(wd, "../testing/assets/manifest.yaml")
			varsDir := filepath.Join(wd, "../testing/assets/vars")

			session, err := act("variable-interpolation", "-m", manifestPath, "-v", varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\n"}`))

			session, err = act("variable-interpolation", "-m", manifestPath, "-v", varsDir)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`{"manifest.yaml":"instance-group:\\n  key1: |\\n    baz\\n  key2: |\\n    foo\\n  key3: |\\n    bar\\npassword: |\\n  fake-password\\n"}`))

		})
	})
})
