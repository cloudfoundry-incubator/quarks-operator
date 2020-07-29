package reference

import (
	"context"

	res "code.cloudfoundry.org/quarks-operator/pkg/kube/util/resources"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"github.com/pkg/errors"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetQJobsReferencedBy returns a list of all QJob referenced by a BOSHDeployment
// The object can be an QuarksStatefulSet or a BOSHDeployment
func GetQJobsReferencedBy(ctx context.Context, client crc.Client, bdpl bdv1.BOSHDeployment) (map[string]bool, error) {

	bdplQjobs := map[string]bool{}
	list, err := res.ListQjobs(ctx, client, bdpl.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "failed getting QJob List")
	}

	for _, qjob := range list.Items {
		if qjob.GetLabels()[bdv1.LabelDeploymentName] == bdpl.Name {
			bdplQjobs[qjob.GetName()] = qjob.Status.Completed
		}
	}

	return bdplQjobs, nil
}
