package statefulset_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/statefulset"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

type StatefulSetEmulation struct {
	pods         []corev1.Pod
	deleted      []bool
	statefulSet  appsv1.StatefulSet
	readyPending int32
	failed       bool
}

func NewStatefulSetEmulation(replicas int) *StatefulSetEmulation {
	sse := &StatefulSetEmulation{
		readyPending: -1,
		deleted:      make([]bool, 4),
		statefulSet: appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Generation:  0,
				Name:        "foo",
				Namespace:   "test",
				Annotations: make(map[string]string),
				Labels:      make(map[string]string),
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointers.Int32(int32(replicas)),
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
					RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
						Partition: pointers.Int32(0),
					},
				},
			},
		},
	}
	sse.Update()
	return sse
}

// Reconciles returns old statefulset if something has changed, nil otherwise
func (sse *StatefulSetEmulation) Reconcile() *event.UpdateEvent {
	var old appsv1.StatefulSet
	sse.statefulSet.DeepCopyInto(&old)
	updateEvent := event.UpdateEvent{ObjectOld: &old, ObjectNew: &sse.statefulSet}
	if sse.readyPending != -1 {
		if sse.failed {
			sse.setPodFailed(&sse.pods[sse.readyPending])
			return nil
		}
		sse.statefulSet.Status.ReadyReplicas++
		sse.setPodReady(&sse.pods[sse.readyPending])
		sse.readyPending = -1
		return &updateEvent
	}
	if sse.statefulSet.Status.Replicas < *sse.statefulSet.Spec.Replicas {
		sse.pods = append(sse.pods, sse.configurePod(len(sse.pods)))
		sse.statefulSet.Status.UpdatedReplicas++
		sse.readyPending = int32(len(sse.pods) - 1)
		sse.statefulSet.Status.Replicas = int32(len(sse.pods))
		sse.statefulSet.Status.Replicas = int32(len(sse.pods))
		return &updateEvent
	}
	if sse.statefulSet.Status.UpdatedReplicas < *sse.statefulSet.Spec.Replicas {
		for i := int32(len(sse.pods)) - 1; *sse.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition <= i; i-- {
			if sse.pods[i].Spec.Subdomain != sse.statefulSet.Spec.Template.Spec.Subdomain {
				sse.pods[i] = sse.configurePod(int(i))
				sse.statefulSet.Status.ReadyReplicas--
				sse.statefulSet.Status.UpdatedReplicas++
				sse.readyPending = i
				sse.deleted[i] = false
				return &updateEvent
			}
		}
		return &updateEvent
	}
	return nil
}
func (sse *StatefulSetEmulation) Request() reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: sse.statefulSet.Name, Namespace: sse.statefulSet.Namespace}}

}

type UpdateOpts func(sse *StatefulSetEmulation)

func WithReplicas(replicas int32) UpdateOpts {
	return func(sse *StatefulSetEmulation) {
		sse.deleted = make([]bool, replicas)
		sse.statefulSet.Spec.Replicas = pointers.Int32(replicas)
	}
}

func WithFailure() UpdateOpts {
	return func(sse *StatefulSetEmulation) {
		sse.failed = true
	}
}

func (sse *StatefulSetEmulation) Update(updateOpts ...UpdateOpts) *event.UpdateEvent {
	var old appsv1.StatefulSet
	sse.statefulSet.DeepCopyInto(&old)
	sse.statefulSet.Status.UpdatedReplicas = 0
	sse.statefulSet.Generation++
	sse.statefulSet.Status.CurrentRevision = sse.statefulSet.Status.UpdateRevision
	sse.statefulSet.Status.UpdateRevision = fmt.Sprintf("generation-%d", sse.statefulSet.Generation)
	sse.statefulSet.Spec.Template.Spec.Subdomain = fmt.Sprintf("generation-%d", sse.statefulSet.Generation)
	if sse.readyPending != -1 {
		sse.readyPending = -1
		sse.statefulSet.Status.ReadyReplicas++
	}
	sse.failed = false
	for _, updateOpt := range updateOpts {
		updateOpt(sse)
	}
	statefulset.ConfigureStatefulSetForRollout(&sse.statefulSet)
	sse.statefulSet.Annotations[statefulset.AnnotationCanaryWatchTime] = "100000"
	sse.statefulSet.Annotations[statefulset.AnnotationUpdateWatchTime] = "100000"
	sse.statefulSet.Annotations[statefulset.AnnotationUpdateStartTime] = strconv.FormatInt(time.Now().Unix(), 10)

	updateEvent := event.UpdateEvent{ObjectOld: &old, ObjectNew: &sse.statefulSet}
	return &updateEvent
}

func (sse *StatefulSetEmulation) configurePod(index int) corev1.Pod {
	return corev1.Pod{
		Spec: sse.statefulSet.Spec.Template.Spec,
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", sse.statefulSet.Name, index),
			Namespace: sse.statefulSet.Namespace,
		},
	}
}
func (sse *StatefulSetEmulation) setPodReady(pod *corev1.Pod) {
	pod.Labels = map[string]string{appsv1.StatefulSetRevisionLabel: sse.statefulSet.Status.UpdateRevision}

	pod.Status = corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
}

func (sse *StatefulSetEmulation) setPodFailed(pod *corev1.Pod) {
	pod.Status = corev1.PodStatus{}
}

func (sse *StatefulSetEmulation) FakeClient() *cfakes.FakeClient {
	client := &cfakes.FakeClient{}

	client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
		switch object := object.(type) {
		case *appsv1.StatefulSet:
			sse.statefulSet.DeepCopyInto(object)
			return nil
		case *corev1.Pod:
			for _, pod := range sse.pods {
				if pod.Name == nn.Name && pod.Namespace == nn.Namespace {
					pod.DeepCopyInto(object)
					return nil
				}
			}
		}
		return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
	})

	client.UpdateCalls(func(ctx context.Context, object runtime.Object, option ...k8sclient.UpdateOption) error {
		object.(*appsv1.StatefulSet).DeepCopyInto(&sse.statefulSet)
		return nil
	})

	client.DeleteCalls(func(ctx context.Context, object runtime.Object, option ...k8sclient.DeleteOption) error {
		switch object := object.(type) {
		case *corev1.Pod:
			for i, pod := range sse.pods {
				if pod.Name == object.Name && pod.Namespace == object.Namespace {
					sse.deleted[i] = true
					return nil
				}
			}
		}
		return apierrors.NewNotFound(schema.GroupResource{}, "???")
	})
	return client
}
