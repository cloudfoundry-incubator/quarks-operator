package integration_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulsetcontroller/v1alpha1"
)

var _ = Describe("ExtendedStatefulSet", func() {
	var (
		extendedStatefulSet essv1.ExtendedStatefulSet
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			extendedStatefulSet = env.DefaultExtendedStatefulSet("testess")
		})

		It("should create a statefulset and eventually a pod", func() {
			// Create an ExtendedStatefulSet
			var ess *essv1.ExtendedStatefulSet
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer tearDown()

			// check for pod
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update a statefulset", func() {
			// Create an ExtendedStatefulSet
			var ess *essv1.ExtendedStatefulSet
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer tearDown()

			// check for pod
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())

			// Update the ExtendedStatefulSet
			ess.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer tearDown()

			// check for pod
			err = env.WaitForPods(env.Namespace, "testpodupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			// check that old statefulset is deleted
		})

		FIt("should do nothing if nothing has changed", func() {
			// Create an ExtendedStatefulSet
			var ess *essv1.ExtendedStatefulSet

			b, _ := json.Marshal(extendedStatefulSet)
			fmt.Println(string(b))

			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer tearDown()

			// check for pod
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())

			// Update the ExtendedStatefulSet
			ess.Labels = map[string]string{
				"essupdated": "yes",
			}
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer tearDown()

			// check for pod
			err = env.WaitForExtendedStatefulSets(env.Namespace, "essupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			expectedMsg := fmt.Sprintf("StatefulSet 'testess-v1' for ExtendedStatefulSet '%s/testess' has not changed, checking if any other changes are necessary.", env.Namespace)
			msgs := env.LogRecorded.FilterMessage(expectedMsg)
			Expect(msgs.Len()).NotTo(Equal(0))
		})
	})
})
