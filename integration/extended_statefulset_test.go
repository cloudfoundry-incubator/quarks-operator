package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("ExtendedStatefulSet", func() {
	var (
		extendedStatefulSet                estsv1.ExtendedStatefulSet
		wrongExtendedStatefulSet           estsv1.ExtendedStatefulSet
		ownedReferencesExtendedStatefulSet estsv1.ExtendedStatefulSet
	)

	BeforeEach(func() {
		essName := fmt.Sprintf("testess-%s", helper.RandString(5))
		extendedStatefulSet = env.DefaultExtendedStatefulSet(essName)

		wrongEssName := fmt.Sprintf("wrong-testess-%s", helper.RandString(5))
		wrongExtendedStatefulSet = env.WrongExtendedStatefulSet(wrongEssName)

		ownedRefEssName := fmt.Sprintf("owned-ref-testess-%s", helper.RandString(5))
		ownedReferencesExtendedStatefulSet = env.OwnedReferencesExtendedStatefulSet(ownedRefEssName)

	})

	AfterEach(func() {
		Expect(env.WaitForPodsDelete(env.Namespace)).To(Succeed())
		// Skipping wait for PVCs to be deleted until the following is fixed
		// https://www.pivotaltracker.com/story/show/166896791
		// Expect(env.WaitForPVCsDelete(env.Namespace)).To(Succeed())
	})

	Context("when correctly setup", func() {
		It("should create a statefulSet and eventually a pod", func() {
			By("Creating an ExtendedStatefulSet")
			var ess *estsv1.ExtendedStatefulSet
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a new version", func() {
			By("Creating an ExtendedStatefulSet")
			var ess *estsv1.ExtendedStatefulSet
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ExtendedStatefulSet")
			ess.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpodupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			statefulSetName := fmt.Sprintf("%s-v%d", ess.GetName(), 1)

			By("Checking for the first version statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should keep two versions if all are not running", func() {
			By("Creating an ExtendedStatefulSet")
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, wrongExtendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPodFailures(env.Namespace, "wrongpod=yes")
			Expect(err).NotTo(HaveOccurred())

			ess, err = env.GetExtendedStatefulSet(env.Namespace, ess.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))

			By("Updating the ExtendedStatefulSet")
			ess.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPodFailures(env.Namespace, "testpodupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			By("Checking that old statefulset is not deleted")
			_, err = env.GetStatefulSet(env.Namespace, ess.GetName()+"-v1")
			Expect(err).NotTo(HaveOccurred())
		})

		It("create a new version if references are updated", func() {
			By("Creating references")
			configMap1 := env.DefaultConfigMap("example1")
			tearDown, err := env.CreateConfigMap(env.Namespace, configMap1)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			configMap2 := env.DefaultConfigMap("example2")
			tearDown, err = env.CreateConfigMap(env.Namespace, configMap2)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			secret1 := env.DefaultSecret("example1")
			tearDown, err = env.CreateSecret(env.Namespace, secret1)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			secret2 := env.DefaultSecret("example2")
			tearDown, err = env.CreateSecret(env.Namespace, secret2)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Creating an ExtendedStatefulSet")
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, ownedReferencesExtendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

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
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			if s2.StringData == nil {
				s2.StringData = make(map[string]string)
			}
			s2.StringData["key1"] = "modified"
			_, tearDown, err = env.UpdateSecret(env.Namespace, *s2)
			Expect(err).ToNot(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking new generation of statefulSet appears")
			err = env.WaitForPods(env.Namespace, "referencedpod=yes")
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPod(env.Namespace, ess.GetName()+"-v1-0")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when volumeClaimTemplates are specified", func() {
		BeforeEach(func() {

			essName := fmt.Sprintf("testess-%s", helper.RandString(5))
			extendedStatefulSet = env.ExtendedStatefulSetWithPVC(essName, "pvc")

		})

		It("Should append the volumeManagement persistent volume claim always even when spec is updated", func() {

			By("Creating an ExtendedStatefulSet")
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName := fmt.Sprintf("%s-v%d", ess.GetName(), 1)

			By("Waiting for the statefulSet")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			volumeManagementStatefulSetName := fmt.Sprintf("%s-%s", "volume-management", ess.GetName())

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ExtendedStatefulSet to v2")
			ess, err = env.GetExtendedStatefulSet(env.Namespace, ess.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))

			*ess.Spec.Template.Spec.Replicas += 1
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 2)

			By("Waiting for the statefulSet")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName := fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 0)

			By("Checking pod v2-0 volumeClaim name")
			pod, err := env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim := fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 1)

			By("Checking pod v2-1 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 1)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the statefulSet to v3")
			essUpdated, err = env.GetExtendedStatefulSet(env.Namespace, essUpdated.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))

			essUpdated.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			essUpdated, tearDown, err = env.UpdateExtendedStatefulSet(env.Namespace, *essUpdated)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 3)

			By("Waiting for the statefulSet")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 3, 0)

			By("Checking pod v3-0 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 3, 1)

			By("Checking pod v3-1 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 1)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the statefulSet to v4")
			essUpdated, err = env.GetExtendedStatefulSet(env.Namespace, essUpdated.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))

			*essUpdated.Spec.Template.Spec.Replicas -= 1
			essUpdated, tearDown, err = env.UpdateExtendedStatefulSet(env.Namespace, *essUpdated)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 4)

			By("Waiting for the statefulSet")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 4, 0)

			By("Checking pod v4-0 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should append the voluMemanagement persistent volume claim when the replicas are increased twice", func() {
			Skip("this test takes too long, skipping until we fix it")
			By("Creating an ExtendedStatefulSet")
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName := fmt.Sprintf("%s-v%d", ess.GetName(), 1)

			By("Waiting for the statefulset")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			volumeManagementStatefulSetName := fmt.Sprintf("%s-%s", "volume-management", ess.GetName())

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ExtendedStatefulSet to v2")
			Expect(env.WaitForLogMsg(env.ObservedLogs, "Updating ExtendedStatefulSet")).To(Succeed())
			ess, err = env.GetExtendedStatefulSet(env.Namespace, ess.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))

			*ess.Spec.Template.Spec.Replicas += 1
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 2)

			By("Waiting for the statefulset")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName := fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 0)

			By("Checking pod v2-0 volumeClaim name")
			pod, err := env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim := fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 1)

			By("Checking pod v2-1 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 1)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the statefulSet to v3")
			Expect(env.WaitForLogMsg(env.ObservedLogs, "Updating ExtendedStatefulSet")).To(Succeed())
			essUpdated, err = env.GetExtendedStatefulSet(env.Namespace, essUpdated.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))

			*essUpdated.Spec.Template.Spec.Replicas += 2
			essUpdated, tearDown, err = env.UpdateExtendedStatefulSet(env.Namespace, *essUpdated)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 3)

			By("Waiting for the statefulset")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 3, 0)

			By("Checking pod v3-0 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 3, 1)

			By("Checking pod v3-1 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 1)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 3, 2)

			By("Checking pod v3-2 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 2)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

		})

		It("Should append the volumeManagement persistent volume claim when the replicas are decreased twice", func() {
			Skip("this test takes too long, skipping until we fix it")

			By("Creating an ExtendedStatefulSet")
			*extendedStatefulSet.Spec.Template.Spec.Replicas = 4
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName := fmt.Sprintf("%s-v%d", ess.GetName(), 1)

			By("Waiting for the statefulSet")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			volumeManagementStatefulSetName := fmt.Sprintf("%s-%s", "volume-management", ess.GetName())

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ExtendedStatefulSet to v2")
			Expect(env.WaitForLogMsg(env.ObservedLogs, "Updating ExtendedStatefulSet")).To(Succeed())
			ess, err = env.GetExtendedStatefulSet(env.Namespace, ess.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))

			*ess.Spec.Template.Spec.Replicas -= 1
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 2)

			By("Waiting for the statefulSet")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName := fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 0)

			By("Checking pod v2-0 volumeClaim name")
			pod, err := env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim := fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 1)

			By("Checking pod v2-1 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 1)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 2, 2)

			By("Checking pod v2-2 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 2)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the statefulSet to v3")
			Expect(env.WaitForLogMsg(env.ObservedLogs, "Updating ExtendedStatefulSet")).To(Succeed())
			essUpdated, err = env.GetExtendedStatefulSet(env.Namespace, essUpdated.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))

			*essUpdated.Spec.Template.Spec.Replicas -= 2
			essUpdated, tearDown, err = env.UpdateExtendedStatefulSet(env.Namespace, *essUpdated)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName = fmt.Sprintf("%s-v%d", ess.GetName(), 3)

			By("Waiting for the statefulset")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName = fmt.Sprintf("%s-v%d-%d", essUpdated.GetName(), 3, 0)

			By("Checking pod v3-0 volumeClaim name")
			pod, err = env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes = make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim = fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())

		})

		It("VolumeManagement statefulset should be created and deleted after actual statefulset is ready", func() {

			By("Creating an ExtendedStatefulSet")
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			statefulSetName := fmt.Sprintf("%s-v%d", ess.GetName(), 1)

			By("Waiting for the statefulset")
			err = env.WaitForStatefulSet(env.Namespace, statefulSetName)
			Expect(err).NotTo(HaveOccurred())

			podName := fmt.Sprintf("%s-v%d-%d", ess.GetName(), 1, 0)

			pod, err := env.GetPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
			for _, volume := range pod.Spec.Volumes {
				volumes[volume.Name] = volume
			}

			By("Checking the persistent volume claim name")
			PersistentVolumeClaim := fmt.Sprintf("%s-%s-%s-%d", "pvc", "volume-management", ess.GetName(), 0)
			Expect(volumes["pvc"].PersistentVolumeClaim.ClaimName).To(Equal(PersistentVolumeClaim))

			volumeManagementStatefulSetName := fmt.Sprintf("%s-%s", "volume-management", ess.GetName())

			By("Checking that volumeManagement statefulSet is deleted")
			err = env.WaitForStatefulSetDelete(env.Namespace, volumeManagementStatefulSetName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should access same volume from different versions at the same time", func() {

			By("Adding volume write command to pod spec template")
			extendedStatefulSet.Spec.Template.Spec.Template.Spec.Containers[0].Image = "opensuse/leap:15.1"
			extendedStatefulSet.Spec.Template.Spec.Template.Spec.Containers[0].Command = []string{"/bin/bash", "-c", "echo present > /etc/random/presentFile ; sleep 3600"}

			By("Creating an ExtendedStatefulSet")
			ess, tearDown, err := env.CreateExtendedStatefulSet(env.Namespace, extendedStatefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(ess).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpod=yes")
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ExtendedStatefulSet")
			ess.Spec.Template.Spec.Template.Spec.Containers[0].Command = []string{"/bin/bash", "-c", "cat /etc/random/presentFile ; sleep 3600"}
			ess.Spec.Template.Spec.Template.ObjectMeta.Labels["testpodupdated"] = "yes"
			essUpdated, tearDown, err := env.UpdateExtendedStatefulSet(env.Namespace, *ess)
			Expect(err).NotTo(HaveOccurred())
			Expect(essUpdated).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("Checking for pod")
			err = env.WaitForPods(env.Namespace, "testpodupdated=yes")
			Expect(err).NotTo(HaveOccurred())

			podName := fmt.Sprintf("%s-v%d-%d", ess.GetName(), 2, 0)

			out, err := env.GetPodLogs(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(Equal("present\n"))
		})
	})
})
