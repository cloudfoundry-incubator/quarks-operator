package integration_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = Describe("Deploy", func() {
	var (
		pollTimeout  = 30 * time.Second
		pollInterval = 500 * time.Millisecond
	)

	Context("when correctly setup", func() {
		It("should deploy a pod", func() {
			tearDown, err := suite.CreateConfigMap(suite.namespace, suite.DefaultConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			tearDown, err = suite.CreateBOSHDeployment(suite.namespace, suite.DefaultBOSHDeployment("test", "manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer tearDown()

			// check for pod
			err = wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
				pod, err := suite.Clientset.CoreV1().Pods(suite.namespace).Get("diego-pod", v1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}

				if pod.Status.Phase == apiv1.PodRunning {
					return true, nil
				}
				return false, nil
			})

			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from deployment")
		})
	})

})
