package reference

import (
	"context"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	res "code.cloudfoundry.org/quarks-operator/pkg/kube/util/resources"
	"github.com/pkg/errors"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetQSTSReferencedBy returns a list of all QSTS referenced by a BOSHDeployment
// The object can be an QuarksStatefulSet or a BOSHDeployment
func GetQSTSReferencedBy(ctx context.Context, client crc.Client, bdpl bdv1.BOSHDeployment) (map[string]bool, error) {

	bdplSTS := map[string]bool{}
	list, err := res.ListQuarksStatefulSets(ctx, client, bdpl.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "failed getting QSTS List")
	}

	for _, sts := range list.Items {
		if sts.GetLabels()[bdv1.LabelDeploymentName] == bdpl.Name {
			bdplSTS[sts.GetName()] = sts.Status.Ready
		}
	}

	return bdplSTS, nil
}
