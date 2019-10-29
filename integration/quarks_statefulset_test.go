package integration_test

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-utils/pkg/pod"

	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/statefulset"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("QuarksStatefulSet", func() {
	var (
		quarksStatefulSet                   qstsv1a1.QuarksStatefulSet
		quarksStatefulSetWith2Replicas      *qstsv1a1.QuarksStatefulSet
		wrongQuarksStatefulSet              qstsv1a1.QuarksStatefulSet
		wrongQuarksStatefulSetWith2Replicas *qstsv1a1.QuarksStatefulSet
		ownedReferencesQuarksStatefulSet    qstsv1a1.QuarksStatefulSet
	)

	BeforeEach(func() {
		qStsName := fmt.Sprintf("test-qsts-%s", helper.RandString(5))
		quarksStatefulSet = env.DefaultQuarksStatefulSet(qStsName)

		quarksStatefulSetWith2Replicas = quarksStatefulSet.DeepCopy()
		quarksStatefulSetWith2Replicas.Spec.Template.Spec.Replicas = pointers.Int32(2)

		wrongEssName := fmt.Sprintf("wrong-test-qsts-%s", helper.RandString(5))
		wrongQuarksStatefulSet = env.WrongQuarksStatefulSet(wrongEssName)

		ownedRefEssName := fmt.Sprintf("owned-ref-test-qsts-%s", helper.RandString(5))
		ownedReferencesQuarksStatefulSet = env.OwnedReferencesQuarksStatefulSet(ownedRefEssName)

		wrongQuarksStatefulSetWith2Replicas = wrongQuarksStatefulSet.DeepCopy()
		wrongQuarksStatefulSetWith2Replicas.Spec.Template.Spec.Replicas = pointers.Int32(2)
		wrongQuarksStatefulSetWith2Replicas.Spec.Template.Annotations[statefulset.AnnotationUpdateWatchTime] = "30000"
		delete(wrongQuarksStatefulSetWith2Replicas.Spec.Template.Annotations, statefulset.AnnotationCanaryWatchTime)
	})

	AfterEach(func() {
		Expect(env.WaitForPodsDelete(env.Namespace)).To(Succeed())
		// Skipping wait for PVCs to be deleted until the following is fixed
		// https://www.pivotaltracker.com/story/show/166896791
		// Expect(env.WaitForPVCsDelete(env.Namespace)).To(Succeed())
	})

	Context("when correctly setup", func() {
		It("should create a statefulSet and eventually a pod", func() {
			By("Creating an QuarksStatefulSet")
			var qSts *qstsv1a1.QuarksStatefulSet
			qSts, tearDown, err := env.CreateQuarksStatefulSet(env.Namespace, quarksStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a new version", func() {
			By("Creating an QuarksStatefulSet")
			var qSts *qstsv1a1.QuarksStatefulSet
			qSts, tearDown, err := env.CreateQuarksStatefulSet(env.Namespace, quarksStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())

			qSts, err = env.GetQuarksStatefulSet(env.Namespace, qSts.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))

			By("Updating the QuarksStatefulSet")
			qSts.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			qStsUpdated, tearDown, err := env.UpdateQuarksStatefulSet(env.Namespace, *qSts)
			Expect(err).NotTo(HaveOccurred())
			Expect(qStsUpdated).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpodupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			statefulSetName := fmt.Sprintf("%s-v%d", qSts.GetName(), 1)

			By("Checking for the first version statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should keep two versions if all are not running", func() {
			By("Creating an QuarksStatefulSet")
			qSts, tearDown, err := env.CreateQuarksStatefulSet(env.Namespace, wrongQuarksStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPodFailures(env.Namespace, "wrongpod=yes")
			Expect(err).NotTo(HaveOccurred())

			qSts, err = env.GetQuarksStatefulSet(env.Namespace, qSts.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))

			sts, err := env.GetStatefulSet(env.Namespace, qSts.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Canary"))

			err = wait.PollImmediate(5*time.Second, 35*time.Second, func() (bool, error) {
				sts, err := env.GetStatefulSet(env.Namespace, qSts.GetName())
				if err != nil {
					return false, err
				}
				if sts.Annotations["quarks.cloudfoundry.org/canary-rollout"] == "Failed" {
					return true, nil
				}
				return false, nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Updating the QuarksStatefulSet")
			qSts.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			qStsUpdated, tearDown, err := env.UpdateQuarksStatefulSet(env.Namespace, *qSts)
			Expect(err).NotTo(HaveOccurred())
			Expect(qStsUpdated).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPodFailures(env.Namespace, "testpodupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			By("Checking that old statefulset is not deleted")
			_, err = env.GetStatefulSet(env.Namespace, qSts.GetName())
			Expect(err).NotTo(HaveOccurred())
		})

		It("create a new version if references are updated", func() {
			By("Creating references")
			configMap1 := env.DefaultConfigMap("example1")
			tearDown, err := env.CreateConfigMap(env.Namespace, configMap1)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			configMap2 := env.DefaultConfigMap("example2")
			tearDown, err = env.CreateConfigMap(env.Namespace, configMap2)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			secret1 := env.DefaultSecret("example1")
			tearDown, err = env.CreateSecret(env.Namespace, secret1)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			secret2 := env.DefaultSecret("example2")
			tearDown, err = env.CreateSecret(env.Namespace, secret2)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Creating an QuarksStatefulSet")
			qSts, tearDown, err := env.CreateQuarksStatefulSet(env.Namespace, ownedReferencesQuarksStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "referencedpod=yes")
			Expect(err).NotTo(HaveOccurred())

			By("Updating one ConfigMap and one Secret")
			cm1, err := env.GetConfigMap(env.Namespace, configMap1.Name)
			Expect(err).ToNot(HaveOccurred())
			s2, err := env.GetSecret(env.Namespace, secret2.Name)
			Expect(err).ToNot(HaveOccurred())

			cm1.Data["key1"] = "modified"
			_, tearDown, err = env.UpdateConfigMap(env.Namespace, *cm1)
			Expect(err).ToNot(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			if s2.StringData == nil {
				s2.StringData = make(map[string]string)
			}
			s2.StringData["key1"] = "modified"
			_, tearDown, err = env.UpdateSecret(env.Namespace, *s2)
			Expect(err).ToNot(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking new generation of statefulSet appears")
			err = env.WaitForPods(env.Namespace, "referencedpod=yes")
			Expect(err).NotTo(HaveOccurred())

			// Two update events for one configMap and one secret
			err = env.WaitForPod(env.Namespace, qSts.GetName()+"-0")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should timeout when scaling after update-watch-time", func() {
			By("Creating an QuarksStatefulSet")
			qSts, tearDown, err := env.CreateQuarksStatefulSet(env.Namespace, *wrongQuarksStatefulSetWith2Replicas)
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPodFailures(env.Namespace, "wrongpod=yes")
			Expect(err).NotTo(HaveOccurred())

			qSts, err = env.GetQuarksStatefulSet(env.Namespace, qSts.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))

			sts, err := env.GetStatefulSet(env.Namespace, qSts.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "CanaryUpscale"))

			err = waitForState(env.Namespace, qSts.Name, "Failed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Rollout should stop on failure and recover when fixed", func() {
			By("Creating an QuarksStatefulSet")
			qSts, tearDown, err := env.CreateQuarksStatefulSet(env.Namespace, *quarksStatefulSetWith2Replicas)
			Expect(err).NotTo(HaveOccurred())
			Expect(qSts).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())

			err = waitForState(env.Namespace, qSts.Name, "Done")
			Expect(err).NotTo(HaveOccurred())

			By("Break the QuarksStatefulSet")
			qSts, err = env.GetQuarksStatefulSet(env.Namespace, qSts.Name)
			Expect(err).NotTo(HaveOccurred())
			brokenQuarksStatefulSet := qSts.DeepCopy()
			brokenQuarksStatefulSet.Spec.Template = env.WrongStatefulSet(brokenQuarksStatefulSet.Name)
			brokenQuarksStatefulSet.Spec.Template.Spec.Replicas = pointers.Int32(2)
			qSts, _, err = env.UpdateQuarksStatefulSet(env.Namespace, *brokenQuarksStatefulSet)
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodFailures(env.Namespace, "wrongpod=yes")
			Expect(err).NotTo(HaveOccurred())

			err = waitForState(env.Namespace, qSts.Name, "Failed")
			Expect(err).NotTo(HaveOccurred())

			running, err := env.PodsRunning(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())
			Expect(running).To(BeTrue())

			By("Repair the QuarksStatefulSet")
			qSts, err = env.GetQuarksStatefulSet(env.Namespace, qSts.Name)
			Expect(err).NotTo(HaveOccurred())
			repairedQuarksStatefulSet := qSts.DeepCopy()
			repairedQuarksStatefulSet.Spec.Template = env.DefaultStatefulSet(brokenQuarksStatefulSet.Name)
			repairedQuarksStatefulSet.Spec.Template.Spec.Replicas = pointers.Int32(2)
			qSts, _, err = env.UpdateQuarksStatefulSet(env.Namespace, *repairedQuarksStatefulSet)
			Expect(err).NotTo(HaveOccurred())

			err = waitForState(env.Namespace, qSts.Name, "Done")
			Expect(err).NotTo(HaveOccurred())

			count, err := env.PodCount(env.Namespace, "testpod=yes", func(p v1.Pod) bool {
				return pod.IsPodReady(&p)
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})
	})
})

func waitForState(namespace string, name string, state string) error {
	return wait.PollImmediate(5*time.Second, 35*time.Second, func() (bool, error) {
		sts, err := env.GetStatefulSet(namespace, name)
		if err != nil {
			return false, err
		}
		if sts.Annotations["quarks.cloudfoundry.org/canary-rollout"] == state {
			return true, nil
		}
		return false, nil
	})
}
