package environment

import (
	"os"
	"os/exec"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"

	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuarksJobCmd helps to run the QuarksJob operator in tests
type QuarksJobCmd struct {
	Path string
}

// NewQuarksJobCmd returns the default QuarksJobCmd
func NewQuarksJobCmd() QuarksJobCmd {
	return QuarksJobCmd{}
}

// Build builds the quarks-job operator binary
func (q *QuarksJobCmd) Build() error {
	var err error
	q.Path, err = gexec.Build("code.cloudfoundry.org/quarks-job/cmd")
	return err
}

// Start starts the specified quarks-job in a namespace
func (q *QuarksJobCmd) Start(namespace string) error {
	cmd := exec.Command(q.Path,
		"-n", namespace,
		"-o", "cfcontainerization",
		"-r", "quarks-job",
		"--service-account", "default",
		"-t", quarksJobTag(),
	)
	_, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	return err
}

func quarksJobTag() string {
	version, found := os.LookupEnv("QUARKS_JOB_IMAGE_TAG")
	if !found {
		version = "dev"
	}
	return version
}

// SetupQjobAccount creates the service account for the quarks job
// for testing this is cluster-admin
func (e *Environment) SetupQjobAccount() error {
	// Bind the persist-output service account to the cluster-admin ClusterRole. Notice that the
	// RoleBinding is namespaced as opposed to ClusterRoleBinding which would give the service account
	// unrestricted permissions to any namespace.
	roleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "persist-output-role",
			Namespace: e.Namespace,
		},
		Subjects: []v1.Subject{
			{
				Kind:      v1.ServiceAccountKind,
				Name:      "default",
				Namespace: e.Namespace,
			},
		},
		RoleRef: v1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	rbac := e.Clientset.RbacV1().RoleBindings(e.Namespace)
	if _, err := rbac.Create(roleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "could not create role binding")
		}
	}
	return nil
}
