package environment

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
)

// SetupNamespace creates the namespace and the clientsets and prepares the teardowm
func (e *Environment) SetupNamespace() error {
	err := e.SetupClientsets()
	if err != nil {
		errors.Wrapf(err, "Integration setup failed. Creating clientsets in %s", e.Namespace)
	}

	nsTeardown, err := e.CreateNamespace(e.Namespace)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Creating namespace %s failed", e.Namespace)
	}

	e.Teardown = func(wasFailure bool) {
		if wasFailure {
			fmt.Println("Collecting debug information...")

			// try to find our dump_env script
			n := 1
			_, filename, _, _ := runtime.Caller(1)
			if idx := strings.Index(filename, "integration/"); idx >= 0 {
				n = strings.Count(filename[idx:], "/")
			}
			var dots []string
			for i := 0; i < n; i++ {
				dots = append(dots, "..")
			}
			dumpCmd := path.Join(append(dots, "testing/dump_env.sh")...)

			out, err := exec.Command(dumpCmd, e.Namespace).CombinedOutput()
			if err != nil {
				fmt.Println("Failed to run the `dump_env.sh` script", err)
			}
			fmt.Println(string(out))
		}

		err := nsTeardown()
		if err != nil {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		if e.Stop != nil {
			close(e.Stop)
		}

		e.removeWebhookCache()
	}

	return nil
}

// removeWebhookCache removes the local webhook config temp folder
func (e *Environment) removeWebhookCache() {
	os.RemoveAll(path.Join(controllers.WebhookConfigDir, controllers.WebhookConfigPrefix+getNamespaceName(e.ID)))
}
