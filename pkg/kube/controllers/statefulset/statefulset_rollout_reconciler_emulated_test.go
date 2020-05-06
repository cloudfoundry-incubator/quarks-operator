package statefulset_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/statefulset"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileStatefulSetRollout", func() {
	var (
		manager   *cfakes.FakeManager
		emulation *StatefulSetEmulation
		ctx       context.Context
		log       *zap.SugaredLogger
		config    *cfcfg.Config
		client    *cfakes.FakeClient
	)

	JustBeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		emulation = NewStatefulSetEmulation(4)
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
	})

	reconciler := func() reconcile.Reconciler {
		client = emulation.FakeClient()
		manager.GetClientReturns(client)
		return statefulset.NewStatefulSetRolloutReconciler(ctx, config, manager)
	}

	reconcile := func(reconciler reconcile.Reconciler, ev *event.UpdateEvent) {
		if ev == nil {
			return
		}
		if statefulset.CheckUpdate(*ev) {
			_, err := reconciler.Reconcile(emulation.Request())
			Expect(err).NotTo(HaveOccurred())
		}

	}

	Context("Initial startup", func() {
		It("startup as expected", func() {
			r := reconciler()
			reconcile(r, emulation.Update())
			Expect(emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate).NotTo(BeNil())

			Expect(emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).NotTo(BeNil())
			Expect(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).To(BeEquivalentTo(0))

			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "CanaryUpscale"))
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))
			Expect(client.UpdateCallCount()).To(Equal(2)) // overall count including the one from 4 lines above
		})

		It("if startup fails and it can be recovered by update", func() {
			r := reconciler()
			reconcile(r, emulation.Update(WithFailure()))
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}

			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "CanaryUpscale"))

			r = reconciler()
			reconcile(r, emulation.Update())
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))
		})
	})

	Context("Update", func() {
		It("rollout as expected", func() {
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
			}
			By("Update ")
			r := reconciler()
			reconcile(r, emulation.Update())
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Canary"))
			Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(3))

			for i := 3; i > 0; i-- {
				By(fmt.Sprintf("pod %d is restarted", i))
				r = reconciler()
				reconcile(r, emulation.Reconcile())
				Expect(client.UpdateCallCount()).To(Equal(0))

				By(fmt.Sprintf("pod %d gets ready", i))
				r = reconciler()
				reconcile(r, emulation.Reconcile())
				Expect(client.UpdateCallCount()).To(Equal(1))
				Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Rollout"))
				Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(i - 1))
			}

			By("pod 0 is restarted")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(0))

			By("pod 0 gets ready")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))
			Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(0))

			By("done")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(0))
		})
	})

	Context("Failed Update", func() {
		It("It recovers from failed update", func() {
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
			}
			By("Failed Update ")
			r := reconciler()
			emulation.Update(WithFailure())
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Canary"))
			Expect(emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate).NotTo(BeNil())
			Expect(emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).NotTo(BeNil())
			Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(3))

			By("pod 3 is restarted")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(0))

			By("pod 3 doesn't get ready")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(0))

			By("Update to new version")
			r = reconciler()

			reconcile(r, emulation.Update())
			Expect(client.DeleteCallCount()).To(Equal(1))
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Canary"))
			Expect(emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate).NotTo(BeNil())
			Expect(emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).NotTo(BeNil())
			Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(3))

			for i := 3; i > 0; i-- {
				By(fmt.Sprintf("pod %d is restarted", i))
				r = reconciler()
				reconcile(r, emulation.Reconcile())
				Expect(client.UpdateCallCount()).To(Equal(0))

				By(fmt.Sprintf("pod %d gets ready", i))
				r = reconciler()
				reconcile(r, emulation.Reconcile())
				Expect(client.UpdateCallCount()).To(Equal(1))
				Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Rollout"))
				Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(i - 1))
			}

		})

	})

	Context("CanaryUpscale", func() {
		It("rescales as expected", func() {
			By("Startup")
			r := reconciler()
			reconcile(r, emulation.Reconcile())
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}

			By("Do rescale")
			r = reconciler()
			reconcile(r, emulation.Update(WithReplicas(5)))
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "CanaryUpscale"))
			Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(4))

			By("pod 4 is started")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(0))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "CanaryUpscale"))

			By("pod 4 is gets ready")
			r = reconciler()
			reconcile(r, emulation.Reconcile())
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Rollout"))

			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))

		})

		It("if rescale fails it can be recovered by update", func() {
			By("Startup")
			r := reconciler()

			reconcile(r, emulation.Reconcile())
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}

			By("Do rescale with failure")
			r = reconciler()
			reconcile(r, emulation.Update(WithReplicas(5), WithFailure()))
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}
			Expect(client.UpdateCallCount()).To(Equal(1))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "CanaryUpscale"))
			Expect(int(*emulation.statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)).To(Equal(4))

			By("Do update")
			r = reconciler()
			reconcile(r, emulation.Update())
			for ev := emulation.Reconcile(); ev != nil; ev = emulation.Reconcile() {
				reconcile(r, ev)
			}
			Expect(client.UpdateCallCount()).To(Equal(6))
			Expect(emulation.statefulSet.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout", "Done"))
		})
	})
})
