package statefulset_test

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/statefulset"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileStatefulSetRollout", func() {
	var (
		manager            *cfakes.FakeManager
		reconciler         reconcile.Reconciler
		ctx                context.Context
		log                *zap.SugaredLogger
		config             *cfcfg.Config
		client             *cfakes.FakeClient
		readyPod           *corev1.Pod
		noneReadyPod       *corev1.Pod
		statefulSet        *appsv1.StatefulSet
		replicas           int32
		partition          int32
		readyReplicas      int32
		updatedReplicas    int32
		updatedStatefulSet appsv1.StatefulSet
	)
	annotations := make(map[string]string)
	timeout := 10 * time.Second
	timeoutTolerance := 5 * time.Second

	BeforeEach(func() {
		annotations[statefulset.AnnotationCanaryRollout] = "Pending"
		annotations[statefulset.AnnotationCanaryWatchTime] = strconv.FormatInt(timeout.Milliseconds(), 10)
		annotations[statefulset.AnnotationUpdateWatchTime] = strconv.FormatInt(timeout.Milliseconds(), 10)
		annotations[statefulset.AnnotationUpdateStartTime] = strconv.FormatInt(time.Now().Unix(), 10)
		replicas = 2
		readyReplicas = 2
		updatedReplicas = 0
		partition = 0
	})

	JustBeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		statefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:               "foo",
						Kind:               "QuarksStatefulSet",
						UID:                "",
						Controller:         pointers.Bool(true),
						BlockOwnerDeletion: pointers.Bool(true),
					},
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointers.Int32(replicas),
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
						Partition: &partition,
					},
				},
			},
			Status: appsv1.StatefulSetStatus{
				CurrentRevision: "1",
				Replicas:        replicas,
				ReadyReplicas:   readyReplicas,
				UpdatedReplicas: updatedReplicas,
			},
		}
		readyPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo-0",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
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
		noneReadyPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   "default",
				UID:         "",
				Annotations: annotations,
			},
		}

		client = &cfakes.FakeClient{}

		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *appsv1.StatefulSet:
				statefulSet.DeepCopyInto(object)
				return nil
			case *corev1.Pod:
				if replicas != readyReplicas && noneReadyPod.Name == nn.Name {
					noneReadyPod.DeepCopyInto(object)
				} else {
					readyPod.DeepCopyInto(object)
				}
				object.Name = nn.Name
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})

		client.UpdateCalls(func(ctx context.Context, object runtime.Object, option ...k8sclient.UpdateOption) error {
			object.(*appsv1.StatefulSet).DeepCopyInto(&updatedStatefulSet)
			return nil
		})

		manager.GetClientReturns(client)
		reconciler = statefulset.NewStatefulSetRolloutReconciler(ctx, config, manager)
	})

	Context("if stateful set gets updated", func() {
		Context("in rollout state 'Canary'", func() {
			BeforeEach(func() {
				annotations["quarks.cloudfoundry.org/canary-rollout"] = "Canary"
			})

			Context("replica=3, updatedReplica=1", func() {
				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-1", Namespace: "default"}}

				BeforeEach(func() {
					replicas = 3
					readyReplicas = replicas
					updatedReplicas = 1
					partition = replicas - 1
				})

				When("and canary_watch_time is exceeded", func() {
					BeforeEach(func() {
						annotations[statefulset.AnnotationCanaryWatchTime] = "-1"
					})
					It("sets state to 'Failed'", func() {
						request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
						_, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Failed"))
					})
				})

				It("the partition is decreased by 1", func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RequeueAfter).To(And(BeNumerically("<=", timeout), BeNumerically(">", timeout-timeoutTolerance)))
					Expect(updatedStatefulSet.Spec.UpdateStrategy.RollingUpdate).NotTo(BeNil())
					Expect(updatedStatefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).NotTo(BeNil())
					Expect(*updatedStatefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).To(BeEquivalentTo(replicas - 2))
				})

				It("the stateful set is updated", func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RequeueAfter).To(And(BeNumerically("<=", timeout), BeNumerically(">", timeout-timeoutTolerance)))
					Expect(client.UpdateCallCount()).To(Equal(1))
					Expect(updatedStatefulSet.ObjectMeta.OwnerReferences).To(Equal(statefulSet.ObjectMeta.OwnerReferences))
				})

				It("the stateful rollout state is now 'Rollout'", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Rollout"))
				})
			})

			Context("readyReplica=2, updatedReplica=2", func() {
				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-0", Namespace: "default"}}

				It("the partition is not set to zero", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(*updatedStatefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).NotTo(Equal(0))
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))
				})
			})
		})

		Context("in rollout state 'CanaryUpscale'", func() {
			BeforeEach(func() {
				annotations["quarks.cloudfoundry.org/canary-rollout"] = "CanaryUpscale"
			})

			When("and update_watch_time is exceeded", func() {
				BeforeEach(func() {
					annotations[statefulset.AnnotationUpdateWatchTime] = "-1"
				})
				It("sets state to 'Failed'", func() {
					request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Failed"))
				})
			})
		})

		Context("in rollout state 'Pending'", func() {
			BeforeEach(func() {
				annotations[statefulset.AnnotationCanaryRollout] = "Pending"
			})

			Context("with a canary_watch_time", func() {
				It("retriggers a reconcile in canary_watch_time ms", func() {
					request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
					response, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.RequeueAfter).To(And(BeNumerically("<=", timeout), BeNumerically(">", timeout-timeoutTolerance)))
				})
			})

			Context("readyReplica=1, updatedReplica=1", func() {
				request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-1", Namespace: "default"}}

				BeforeEach(func() {
					replicas = 2
					readyReplicas = 1
					updatedReplicas = 1
					partition = 1
				})

				It("the stateful rollout state is now 'Canary'", func() {
					noneReadyPod.Name = "foo-1"
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(client.UpdateCallCount()).To(Equal(1))
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Canary"))
				})
			})
		})

		Context("in rollout state 'Rollout'", func() {
			request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
			BeforeEach(func() {
				annotations[statefulset.AnnotationCanaryRollout] = "Rollout"
			})

			When("all replicas are ready", func() {

				BeforeEach(func() {
					readyReplicas = 3
					replicas = 3
					updatedReplicas = 3
				})

				It("the stateful rollout state is now 'Done'", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(client.UpdateCallCount()).To(Equal(1))
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))
				})
			})

			When("NOT all replicas are ready", func() {
				BeforeEach(func() {
					readyReplicas = 2
					replicas = 3
					updatedReplicas = 2
					partition = 1
				})

				It("some pod is not ready, rollout continues", func() {
					noneReadyPod.Name = "foo-0"
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(client.UpdateCallCount()).To(Equal(1))
					Expect(client.DeleteCallCount()).To(Equal(1))
				})

				It("last updated pod is not ready, rollout stops", func() {
					noneReadyPod.Name = "foo-1"
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(client.UpdateCallCount()).To(Equal(0))
					Expect(client.DeleteCallCount()).To(Equal(0))
				})

				When("update fails", func() {
					JustBeforeEach(func() {
						client.UpdateReturns(errors.New("injected error"))
					})

					It("doesn't delete any pods", func() {
						noneReadyPod.Name = "foo-0"
						_, err := reconciler.Reconcile(request)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("injected"))
						Expect(client.UpdateCallCount()).To(Equal(1))
						Expect(client.DeleteCallCount()).To(Equal(0))
					})
				})
			})

			When("rollout starts", func() {

				BeforeEach(func() {
					readyReplicas = 2
					replicas = 3
					updatedReplicas = 1
					partition = 2
				})

				It("state is not changed to canary again", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(client.UpdateCallCount()).To(Equal(1))
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Rollout"))
				})
			})

			When("update_watch_time is exceeded", func() {
				BeforeEach(func() {
					annotations[statefulset.AnnotationUpdateWatchTime] = "-1"
				})
				It("sets state to 'Failed'", func() {
					request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Failed"))
				})
			})
		})

		Context("in rollout state 'Done'", func() {
			request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
			BeforeEach(func() {
				annotations[statefulset.AnnotationCanaryRollout] = "Done"
			})

			When("and canary_watch_time is exceeded", func() {
				BeforeEach(func() {
					annotations[statefulset.AnnotationCanaryRollout] = "Done"
					annotations[statefulset.AnnotationCanaryWatchTime] = "-1"
				})

				It("state remains 'Done'", func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(reconcile.Result{}).To(Equal(result))
					Expect(client.UpdateCallCount()).To(Equal(0))
				})
			})

			When("and update_watch_time is exceeded", func() {
				BeforeEach(func() {
					annotations[statefulset.AnnotationCanaryRollout] = "Done"
					annotations[statefulset.AnnotationUpdateWatchTime] = "-1"
				})

				It("state remains 'Done'", func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(reconcile.Result{}).To(Equal(result))
					Expect(client.UpdateCallCount()).To(Equal(0))
				})
			})

		})
	})
})
