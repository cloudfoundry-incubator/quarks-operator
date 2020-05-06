package operatorimage_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/operatorimage"
)

var _ = Describe("operatorimage", func() {
	Describe("GetOperatorDockerImage", func() {
		It("returns the location of the docker image", func() {
			err := operatorimage.SetupOperatorDockerImage("foo", "bar", "1.2.3", corev1.PullPolicy("Always"))
			Expect(err).ToNot(HaveOccurred())
			Expect(operatorimage.GetOperatorDockerImage()).To(Equal("foo/bar:1.2.3"))
			Expect(operatorimage.GetOperatorImagePullPolicy()).To(Equal(corev1.PullAlways))
		})
	})
})
