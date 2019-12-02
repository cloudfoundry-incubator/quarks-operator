package statefulset_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/statefulset"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("FilterLabels", func() {
	var labels map[string]string

	Context("map of labels", func() {

		BeforeEach(func() {
			labels = make(map[string]string)
			labels[manifest.LabelDeploymentName] = "xxx"
			labels[manifest.LabelDeploymentVersion] = "3"
		})

		It("deployment version is filtered", func() {
			filteredLabels := statefulset.FilterLabels(labels)
			Expect(filteredLabels).NotTo(HaveKey(manifest.LabelDeploymentVersion))
		})
	})

})

var _ = Describe("CleanupNonReadyPod", func() {
	var (
		ctx          context.Context
		log          *zap.SugaredLogger
		client       *cfakes.FakeClient
		statefulSet  *v1beta2.StatefulSet
		readyPod     *corev1.Pod
		pendingPod   *corev1.Pod
		noneReadyPod *corev1.Pod
	)
	annotations := make(map[string]string)

	JustBeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)

		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		selector := map[string]string{
			"quarks.cloudfoundry.org/deployment-name":     "kubecf",
			"quarks.cloudfoundry.org/instance-group-name": "api",
		}
		readyPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo-0",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
				Labels:      selector,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
		pendingPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo-1",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
				Labels:      selector,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}
		noneReadyPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo-2",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
				Labels:      selector,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
		statefulSet = &v1beta2.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:               "foo",
						Kind:               "ExtendedStatefulSet",
						UID:                "",
						Controller:         pointers.Bool(true),
						BlockOwnerDeletion: pointers.Bool(true),
					},
				},
			},
			Spec: v1beta2.StatefulSetSpec{
				Replicas: pointers.Int32(3),
				Selector: &metav1.LabelSelector{MatchLabels: selector},
				UpdateStrategy: v1beta2.StatefulSetUpdateStrategy{
					RollingUpdate: &v1beta2.RollingUpdateStatefulSetStrategy{
						Partition: pointers.Int32(8),
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: readyPod.Spec,
				},
			},
			Status: v1beta2.StatefulSetStatus{
				UpdatedReplicas: 3,
				ReadyReplicas:   1,
				Replicas:        3,
			},
		}

		client = &cfakes.FakeClient{}

		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *corev1.Pod:
				for _, pod := range []*corev1.Pod{readyPod, noneReadyPod, pendingPod} {
					if pod.Name == nn.Name {
						pod.DeepCopyInto(object)
						return nil
					}
				}
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})

	})

	Context("statefulset is updated", func() {

		It("ready pod is not deleted", func() {
			err := statefulset.CleanupNonReadyPod(ctx, client, statefulSet, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})
		It("pending pod is deleted", func() {
			err := statefulset.CleanupNonReadyPod(ctx, client, statefulSet, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(1))
			_, pod, _ := client.DeleteArgsForCall(0)
			Expect(pod.(*corev1.Pod).Name).To(Equal(pendingPod.Name))
		})
		It("none ready pod is deleted", func() {
			err := statefulset.CleanupNonReadyPod(ctx, client, statefulSet, 2)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(1))
			_, pod, _ := client.DeleteArgsForCall(0)
			Expect(pod.(*corev1.Pod).Name).To(Equal(noneReadyPod.Name))
		})
		It("out of index pod is working", func() {
			err := statefulset.CleanupNonReadyPod(ctx, client, statefulSet, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

	})

})
