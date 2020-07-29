package boshdeployment_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	bdplcontroller "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileBDPL", func() {
	var (
		manager       *cfakes.FakeManager
		reconciler    reconcile.Reconciler
		jobreconciler reconcile.Reconciler

		request             reconcile.Request
		ctx                 context.Context
		log                 *zap.SugaredLogger
		config              *cfcfg.Config
		client              *cfakes.FakeClient
		bdpl                *bdv1.BOSHDeployment
		desiredQStatefulSet *qstsv1a1.QuarksStatefulSet
		desiredQJob         *qjv1a1.QuarksJob
		reconcileRequest    func()
		status              *cfakes.FakeStatusWriter
	)

	BeforeEach(func() {
		err := controllers.AddToScheme(scheme.Scheme)
		Expect(err).ToNot(HaveOccurred())

		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		reconcileRequest = func() {
			result, err := jobreconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
			result, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		}

		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		status = &cfakes.FakeStatusWriter{}

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *qstsv1a1.QuarksStatefulSet:
				desiredQStatefulSet.DeepCopyInto(object)
				return nil
			case *qjv1a1.QuarksJob:
				desiredQJob.DeepCopyInto(object)
				return nil
			case *bdv1.BOSHDeployment:
				bdpl.DeepCopyInto(object)
				return nil
			}

			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})

		client.ListCalls(func(context context.Context, object runtime.Object, opts ...crc.ListOption) error {
			switch object := object.(type) {
			case *qjv1a1.QuarksJobList:
				list := &qjv1a1.QuarksJobList{Items: []qjv1a1.QuarksJob{*desiredQJob}}
				list.DeepCopyInto(object)
				return nil
			case *qstsv1a1.QuarksStatefulSetList:
				list := &qstsv1a1.QuarksStatefulSetList{Items: []qstsv1a1.QuarksStatefulSet{*desiredQStatefulSet}}
				list.DeepCopyInto(object)
				return nil
			}

			return apierrors.NewNotFound(schema.GroupResource{}, "test")
		})

		manager.GetClientReturns(client)

		client.StatusCalls(func() crc.StatusWriter { return status })
	})

	JustBeforeEach(func() {
		reconciler = bdplcontroller.NewStatusQSTSReconciler(ctx, config, manager)
		jobreconciler = bdplcontroller.NewQJobStatusReconciler(ctx, config, manager)
		desiredQStatefulSet = &qstsv1a1.QuarksStatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
				UID:       "",
				Labels:    map[string]string{bdv1.LabelDeploymentName: "deployment-name"},
			},
			Spec:   qstsv1a1.QuarksStatefulSetSpec{},
			Status: qstsv1a1.QuarksStatefulSetStatus{Ready: false},
		}

		bdpl = &bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment-name",
				Namespace: "default",
			},
			Spec: bdv1.BOSHDeploymentSpec{
				Manifest: bdv1.ResourceReference{
					Name: "dummy-manifest",
					Type: "configmap",
				},
				Ops: []bdv1.ResourceReference{
					{
						Name: "bar",
						Type: "configmap",
					},
					{
						Name: "baz",
						Type: "secret",
					},
				},
			},
		}
		desiredQJob = &qjv1a1.QuarksJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
				UID:       "",
			},
			Spec:   qjv1a1.QuarksJobSpec{},
			Status: qjv1a1.QuarksJobStatus{Completed: false},
		}
		status.UpdateCalls(func(context context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
			switch bdplUpdate := object.(type) {
			case *bdv1.BOSHDeployment:
				bdpl = bdplUpdate
			}

			return nil
		})
	})

	Context("BDPL is in 'converting' state", func() {
		It("updates the BDPL Status", func() {

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))
			Expect(bdpl.Status.TotalJobCount).To(Equal(0))

			Expect(bdpl.Name).To(Equal("deployment-name"))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateConverting))
		})
	})

	Context("BDPL is in 'deployed' state", func() {
		It("updates the bdpl status with the deployed state", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: true}

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateDeployed))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})
	})

	Context("BDPL is in 'deployed' state with jobs that doesn't belong to the deployment", func() {
		It("updates the bdpl status with the deployed state ignoring the qjob", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: true}

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			result, err = jobreconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: false}))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.TotalJobCount).To(Equal(0))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateDeployed))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})

		It("updates the bdpl status with the deployed state ignoring the qjob belonging to another deployment", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: true}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "another-deployment-name"}

			reconcileRequest()

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.TotalJobCount).To(Equal(0))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateDeployed))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})
	})

	Context("BDPL is being converted - ig is ready, jobs running", func() {
		It("updates the bdpl statuswith the converting state", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: true}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "deployment-name"}

			reconcileRequest()

			Expect(bdpl.Status.TotalJobCount).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateConverting))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})

		It("inverting reconciler running order doesn't change result ", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: true}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "deployment-name"}

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			result, err = jobreconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(bdpl.Status.TotalJobCount).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateConverting))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})
	})

	Context("BDPL is resolving - jobs are running ig is not ready yet", func() {
		It("updates the bdpl status with the resolving state", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: false}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "deployment-name"}

			reconcileRequest()

			Expect(bdpl.Status.TotalJobCount).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(0))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateResolving))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})
	})

	Context("BDPL with multiple instance groups", func() {
		BeforeEach(func() {
			client.ListCalls(func(context context.Context, object runtime.Object, opts ...crc.ListOption) error {
				switch object := object.(type) {
				case *qjv1a1.QuarksJobList:
					list := &qjv1a1.QuarksJobList{Items: []qjv1a1.QuarksJob{*desiredQJob}}
					list.DeepCopyInto(object)
					return nil
				case *qstsv1a1.QuarksStatefulSetList:
					list := &qstsv1a1.QuarksStatefulSetList{Items: []qstsv1a1.QuarksStatefulSet{*desiredQStatefulSet, qstsv1a1.QuarksStatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo-2",
							Namespace: "default",
							UID:       "",
							Labels:    map[string]string{bdv1.LabelDeploymentName: "deployment-name"},
						},
						Spec:   qstsv1a1.QuarksStatefulSetSpec{},
						Status: qstsv1a1.QuarksStatefulSetStatus{Ready: true},
					},
					}}
					list.DeepCopyInto(object)
					return nil
				}

				return apierrors.NewNotFound(schema.GroupResource{}, "test")
			})
		})

		It("updates the bdpl status with the resolving state", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: false}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "deployment-name"}

			reconcileRequest()

			Expect(bdpl.Status.TotalJobCount).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(0))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(2))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateResolving))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})

		It("updates the bdpl status with the job state", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: false}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "deployment-name"}
			desiredQJob.Status.Completed = true

			reconcileRequest()

			Expect(bdpl.Status.TotalJobCount).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(1))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(2))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(1))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateConverting))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})

		It("updates the bdpl status with the 'deployed' state", func() {
			desiredQStatefulSet.Status = qstsv1a1.QuarksStatefulSetStatus{Ready: true}
			desiredQJob.Labels = map[string]string{bdv1.LabelDeploymentName: "deployment-name"}
			desiredQJob.Status.Completed = true

			reconcileRequest()

			Expect(bdpl.Status.TotalJobCount).To(Equal(1))
			Expect(bdpl.Status.CompletedJobCount).To(Equal(1))

			Expect(bdpl.Status.TotalInstanceGroups).To(Equal(2))
			Expect(bdpl.Status.DeployedInstanceGroups).To(Equal(2))
			Expect(bdpl.Status.State).To(Equal(bdplcontroller.BDPLStateDeployed))
			Expect(bdpl.Name).To(Equal("deployment-name"))
		})
	})
})
