package boshdeployment_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	cvfakes "code.cloudfoundry.org/cf-operator/pkg/bosh/converter/fakes"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	mfakes "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileBPM", func() {
	var (
		manager                   *cfakes.FakeManager
		reconciler                reconcile.Reconciler
		recorder                  *record.FakeRecorder
		request                   reconcile.Request
		ctx                       context.Context
		resolver                  mfakes.FakeDesiredManifest
		kubeConverter             cvfakes.FakeKubeConverter
		manifest                  *bdm.Manifest
		logs                      *observer.ObservedLogs
		log                       *zap.SugaredLogger
		config                    *cfcfg.Config
		client                    *cfakes.FakeClient
		manifestWithVars          *corev1.Secret
		bpmInformation            *corev1.Secret
		bpmInformationNoProcesses *corev1.Secret
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		recorder = record.NewFakeRecorder(20)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		manager.GetEventRecorderForReturns(recorder)
		resolver = mfakes.FakeDesiredManifest{}
		kubeConverter = cvfakes.FakeKubeConverter{}

		kubeConverter.BPMResourcesReturns(&converter.BPMResources{}, nil)
		size := 1024

		manifest = &bdm.Manifest{
			Name: "fake-manifest",
			Releases: []*bdm.Release{
				{
					Name:    "bar",
					URL:     "docker.io/cfcontainerization",
					Version: "1.0",
					Stemcell: &bdm.ReleaseStemcell{
						OS:      "opensuse",
						Version: "42.3",
					},
				},
			},
			InstanceGroups: []*bdm.InstanceGroup{
				{
					Name:               "fakepod",
					Instances:          1,
					PersistentDisk:     &size,
					PersistentDiskType: "standard",
					Jobs: []bdm.Job{
						{
							Name:    "foo",
							Release: "bar",
							Properties: bdm.JobProperties{
								Properties: map[string]interface{}{
									"password": "((foo_password))",
								},
								Quarks: bdm.Quarks{
									Ports: []bdm.Port{
										{
											Name:     "foo",
											Protocol: "TCP",
											Internal: 8080,
										},
									},
								},
							},
						},
					},
				},
			},
			Variables: []bdm.Variable{
				{
					Name: "foo_password",
					Type: "password",
				},
			},
			DNS: bdm.NewSimpleDomainNameService("fake-manifest"),
		}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		logs, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		ctx = ctxlog.NewContextWithRecorder(ctx, "TestRecorder", recorder)

		manifestWithVars = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo.desired-manifest-v1",
				Namespace: "default",
				Labels: map[string]string{
					bdv1.LabelDeploymentName:             "foo",
					versionedsecretstore.LabelSecretKind: "versionedSecret",
					versionedsecretstore.LabelVersion:    "1",
				},
			},
			Data: map[string][]byte{
				"manifest.yaml": []byte(`director_uuid: ""
instance_groups:
- azs: []
  instances: 1
  jobs:
  - name: foo
    properties:
      quarks:
        bpm: {}
        consumes: {}
        instances: []
        release: ""
      password: generated-password
    release: bar
  name: fakepod
  stemcell: ""
  vm_resources: null
name: testcr
releases:
- name: bar
  version: "1.0"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse
    version: 42.3
variables: []
`),
			},
		}

		bpmInformation = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo.bpm.fakepod",
				Namespace: "default",
				Labels: map[string]string{
					bdv1.LabelDeploymentName:             "foo",
					versionedsecretstore.LabelSecretKind: "versionedSecret",
					versionedsecretstore.LabelVersion:    "1",
					qjv1a1.LabelRemoteID:                 "fakepod",
				},
			},
			Data: map[string][]byte{
				"bpm.yaml": []byte(`foo:
  processes:
  - name: fake
    executable: /var/vcap/packages/fake/bin/fake-exec
    args: []
    limits:
      open_files: 100000`),
			},
		}

		bpmInformationNoProcesses = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo.bpm.fakepod",
				Namespace: "default",
				Labels: map[string]string{
					bdv1.LabelDeploymentName:             "foo",
					versionedsecretstore.LabelSecretKind: "versionedSecret",
					versionedsecretstore.LabelVersion:    "1",
					qjv1a1.LabelRemoteID:                 "fakepod",
				},
			},
			Data: map[string][]byte{
				"bpm.yaml": []byte(``),
			},
		}

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *corev1.Secret:
				if nn.Name == manifestWithVars.Name {
					manifestWithVars.DeepCopyInto(object)
				}
				if nn.Name == bpmInformation.Name {
					bpmInformation.DeepCopyInto(object)
				}
			}

			return nil
		})
		client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
			switch object := object.(type) {
			case *corev1.SecretList:
				secretList := corev1.SecretList{}
				secretList.Items = []corev1.Secret{
					*manifestWithVars,
					*bpmInformation,
				}
				secretList.DeepCopyInto(object)
			}

			return nil
		})

		manager.GetClientReturns(client)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo.bpm.fakepod", Namespace: "default"}}
	})

	JustBeforeEach(func() {
		resolver.DesiredManifestReturns(manifest, nil)
		reconciler = cfd.NewBPMReconciler(ctx, config, manager, &resolver,
			controllerutil.SetControllerReference, &kubeConverter,
		)
	})

	Describe("Reconcile", func() {
		Context("when manifest with ops is created", func() {
			It("handles an error when getting the resource", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						return errors.New("some error")
					}

					return nil
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(5 * time.Second))
				Expect(logs.FilterMessageSnippet("Failed to get Instance Group BPM versioned secret 'default/foo.bpm.fakepod'").Len()).To(Equal(1))
			})

			It("handles an error when applying BPM info", func() {
				kubeConverter.BPMResourcesReturns(&converter.BPMResources{}, errors.New("fake-error"))
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						if nn.Name == request.Name {
							bpmInformationNoProcesses.DeepCopyInto(object)
						}
					}

					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to apply BPM information"))
			})

			It("handles an error when deploying instance groups", func() {
				kubeConverter.BPMResourcesReturns(&converter.BPMResources{
					Services: []corev1.Service{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fake-bpm-svc",
								Labels: map[string]string{
									bdm.LabelInstanceGroupName: "fakepod",
								},
							},
						},
					},
				}, nil)

				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						if nn.Name == manifestWithVars.Name {
							manifestWithVars.DeepCopyInto(object)
						}
						if nn.Name == bpmInformation.Name {
							bpmInformation.DeepCopyInto(object)
						}
					case *corev1.Service:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})

				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					return errors.New("fake-error")
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to start: failed to apply Service for instance group 'fakepod'"))
			})

			It("creates instance groups and updates bpm configs created state to deploying state successfully", func() {
				client.UpdateCalls(func(context context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
					switch object.(type) {
					}
					return nil
				})

				By("From bpm configs created to variable interpolated state")
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue: false,
				}))

				newInstance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, newInstance)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
