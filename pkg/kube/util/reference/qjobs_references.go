package reference

import (
	"context"

	"github.com/pkg/errors"

	crc "sigs.k8s.io/controller-runtime/pkg/client"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// GetQJobsReferencedBy returns a list of all QJob referenced by a BOSHDeployment
// The object can be an QuarksStatefulSet or a BOSHDeployment
func GetQJobsReferencedBy(ctx context.Context, client crc.Client, bdpl bdv1.BOSHDeployment) (map[string]bool, error) {

	bdplQjobs := map[string]bool{}
	list := &qjv1a1.QuarksJobList{}
	err := client.List(ctx, list, crc.InNamespace(bdpl.Namespace))
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
