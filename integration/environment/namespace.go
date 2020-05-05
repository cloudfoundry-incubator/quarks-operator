package environment

import (
	"os"
	"path"
	"strings"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
)

// SetupNamespace creates the namespace and the clientsets and prepares the teardowm
func (e *Environment) SetupNamespace() error {
	nsTeardown, err := e.CreateNamespace(e.Namespace)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Creating namespace %s failed", e.Namespace)
	}

	e.Teardown = func(wasFailure bool) {
		if wasFailure {
			utils.DumpENV(e.Namespace)
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

// NamespaceDeletionInProgress returns true if the error indicates deletion will happen
// eventually
func (e *Environment) NamespaceDeletionInProgress(err error) bool {
	return strings.Contains(err.Error(), "namespace will automatically be purged")
}

// removeWebhookCache removes the local webhook config temp folder
func (e *Environment) removeWebhookCache() {
	os.RemoveAll(path.Join(controllers.WebhookConfigDir, controllers.WebhookConfigPrefix+utils.GetNamespaceName(e.ID)))
}
