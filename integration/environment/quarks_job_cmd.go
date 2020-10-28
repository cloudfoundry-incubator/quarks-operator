package environment

import (
	"context"
	"os"
	"os/exec"
	"strconv"

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

// Build builds the quarks-job operator binary
func (q *QuarksJobCmd) Build() error {
	var err error
	q.Path, err = gexec.Build("code.cloudfoundry.org/quarks-job/cmd")
	return err
}

// Start starts the specified quarks-job in a namespace
func (q *QuarksJobCmd) Start(id string) error {
	cmd := exec.Command(q.Path,
		"-o", "ghcr.io/cloudfoundry-incubator",
		"-r", "quarks-job",
		"-t", quarksJobTag(),
		"--meltdown-duration", strconv.Itoa(defaultTestMeltdownDuration),
		"--meltdown-requeue-after", strconv.Itoa(defaultTestMeltdownRequeueAfter),
		"--monitored-id", id,
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

const persistOutputServiceAccount = "default"

// SetupQjobAccount creates the role binding for the quarks job's persist output feature.
// For testing this is cluster-admin.
func (e *Environment) SetupQjobAccount() error {
	// Bind the persist-output service account to the cluster-admin ClusterRole. Notice that the
	// RoleBinding is namespaced as opposed to ClusterRoleBinding which would give the service account
	// unrestricted permissions to any namespace.
	roleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "persist-output-rb",
			Namespace: e.Namespace,
		},
		Subjects: []v1.Subject{
			{
				Kind:      v1.ServiceAccountKind,
				Name:      persistOutputServiceAccount,
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
	if _, err := rbac.Create(context.Background(), roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "could not create role binding")
		}
	}
	return nil
}
