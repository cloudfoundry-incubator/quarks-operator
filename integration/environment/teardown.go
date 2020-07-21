package environment

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/quarks-utils/pkg/webhook"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
)

// Teardown removes the setup after each test
func Teardown(e *Environment, wasFailure bool) {
	e.Teardown(wasFailure)
	e.removeWebhookCache()
}

// removeWebhookCache removes the local webhook config temp folder
func (e *Environment) removeWebhookCache() {
	os.RemoveAll(path.Join(webhook.ConfigDir, controllers.WebhookConfigPrefix+utils.GetNamespaceName(e.ID)))
}

// NukeWebhooks nukes all webhooks at the end of the run
func NukeWebhooks(namespacesToNuke []string) {
	for _, namespace := range namespacesToNuke {
		err := cmdHelper.DeleteWebhooks(namespace)
		if err != nil {
			fmt.Printf("WARNING: failed to delete mutatingwebhookconfiguration in %s: %v\n", namespace, err)
		}
	}
}
