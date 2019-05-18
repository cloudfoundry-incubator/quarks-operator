package boshdeployment_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
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

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileBPM", func() {
	var (
		manager                       *cfakes.FakeManager
		reconciler                    reconcile.Reconciler
		recorder                      *record.FakeRecorder
		request                       reconcile.Request
		ctx                           context.Context
		resolver                      fakes.FakeResolver
		manifest                      *bdm.Manifest
		log                           *zap.SugaredLogger
		config                        *cfcfg.Config
		client                        *cfakes.FakeClient
		instance                      *bdv1.BOSHDeployment
		manifestWithVars              *corev1.Secret
		instanceGroupResolvedManifest *corev1.Secret
		bpmInformation                *corev1.Secret
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		recorder = record.NewFakeRecorder(20)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		manager.GetRecorderReturns(recorder)
		resolver = fakes.FakeResolver{}

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
					Name:      "fakepod",
					Instances: 1,
					Jobs: []bdm.Job{
						{
							Name:    "foo",
							Release: "bar",
							Properties: bdm.JobProperties{
								Properties: map[string]interface{}{
									"password": "((foo_password))",
								},
								BOSHContainerization: bdm.BOSHContainerization{
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
		}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		ctx = ctxlog.NewContextWithRecorder(ctx, "TestRecorder", recorder)

		instance = &bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: bdv1.BOSHDeploymentSpec{
				Manifest: bdv1.Manifest{
					Ref:  "dummy-manifest",
					Type: "configmap",
				},
				Ops: []bdv1.Ops{
					{
						Ref:  "bar",
						Type: "configmap",
					},
					{
						Ref:  "baz",
						Type: "secret",
					},
				},
			},
			Status: bdv1.BOSHDeploymentStatus{
				State: cfd.BPMConfigsCreatedState,
			},
		}

		manifestWithVars = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo.with-vars.interpolation-v1",
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
      bosh_containerization:
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
		instanceGroupResolvedManifest = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo.ig-resolved.fakepod-v1",
				Namespace: "default",
				Labels: map[string]string{
					bdv1.LabelDeploymentName:             "foo",
					versionedsecretstore.LabelSecretKind: "versionedSecret",
					versionedsecretstore.LabelVersion:    "1",
				},
			},
			Data: map[string][]byte{
				"properties.yaml": []byte(`name: foo
director_uuid: ""
instance_groups:
- name: fakepod
  instances: 1
  azs: []
  jobs:
  - name: foo
    release: bar
    properties:
      bosh_containerization:
        instances:
        - address: fakepod-foo-0.default.svc.cluster.local
          az: ""
          id: fakepod-0-foo
          index: 0
          instance: 0
          name: fakepod-foo
          networks: {}
          ip: ""
        bpm:
          processes:
          - name: fake
            executable: /var/vcap/packages/fake/bin/fake-exec
            args: []
            limits:
              open_files: 100000
      password: generated-password
  vm_resources: null
  stemcell: ""
releases:
- name: bar
  version: "1.0"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse
    version: 42.3`),
			},
		}
		bpmInformation = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo.bpm.fakepod-v1",
				Namespace: "default",
				Labels: map[string]string{
					bdv1.LabelDeploymentName:             "foo",
					versionedsecretstore.LabelSecretKind: "versionedSecret",
					versionedsecretstore.LabelVersion:    "1",
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

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object.(type) {
			case *bdv1.BOSHDeployment:
				instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
			case *corev1.Secret:
				if nn.Name == manifestWithVars.Name {
					manifestWithVars.DeepCopyInto(object.(*corev1.Secret))
				}
				if nn.Name == instanceGroupResolvedManifest.Name {
					instanceGroupResolvedManifest.DeepCopyInto(object.(*corev1.Secret))
				}
				if nn.Name == bpmInformation.Name {
					bpmInformation.DeepCopyInto(object.(*corev1.Secret))
				}
			}

			return nil
		})
		client.ListCalls(func(context context.Context, options *crc.ListOptions, object runtime.Object) error {
			switch object.(type) {
			case *corev1.SecretList:
				secretList := corev1.SecretList{}
				secretList.Items = []corev1.Secret{
					*manifestWithVars,
					*instanceGroupResolvedManifest,
					*bpmInformation,
				}
				secretList.DeepCopyInto(object.(*corev1.SecretList))
			}

			return nil
		})

		manager.GetClientReturns(client)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
	})

	JustBeforeEach(func() {
		resolver.ResolveManifestReturns(manifest, nil)
		reconciler = cfd.NewBPMReconciler(ctx, config, manager, &resolver, controllerutil.SetControllerReference)
	})

	Describe("Reconcile", func() {
		Context("when manifest with ops is created", func() {
			It("handles an error when getting latest instanceGroupResolved manifest", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *corev1.Secret:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("waiting for BPM"))
			})

			It("handles an error when applying BPM info", func() {
				missingInstancesManifest := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo.ig-resolved.fakepod-v1",
						Namespace: "default",
						Labels: map[string]string{
							bdv1.LabelDeploymentName:             "foo",
							versionedsecretstore.LabelSecretKind: "versionedSecret",
							versionedsecretstore.LabelVersion:    "1",
						},
					},
					Data: map[string][]byte{
						"properties.yaml": []byte(`name: foo
director_uuid: ""
instance_groups:
- name: fakepod
  instances: 1
  azs: []
  jobs:
  - name: foo
    release: bar
    properties:
      bosh_containerization:
        bpm:
          processes:
          - name: fake
            executable: /var/vcap/packages/fake/bin/fake-exec
            args: []
            limits:
              open_files: 100000
      password: generated-password
  vm_resources: null
  stemcell: ""
releases:
- name: bar
  version: "1.0"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse
    version: 42.3`),
					},
				}
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *corev1.Secret:
						if nn.Name == missingInstancesManifest.Name {
							missingInstancesManifest.DeepCopyInto(object.(*corev1.Secret))
						}
					}

					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to apply BPM information"))
			})

			It("handles an error when deploying instance groups", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *corev1.Secret:
						if nn.Name == manifestWithVars.Name {
							manifestWithVars.DeepCopyInto(object.(*corev1.Secret))
						}
						if nn.Name == instanceGroupResolvedManifest.Name {
							instanceGroupResolvedManifest.DeepCopyInto(object.(*corev1.Secret))
						}
						if nn.Name == bpmInformation.Name {
							bpmInformation.DeepCopyInto(object.(*corev1.Secret))
						}
					case *corev1.Service:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})

				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					return errors.New("fake-error")
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to deploy instance groups"))
			})

			It("creates instance groups and updates bpm configs created state to deploying state successfully", func() {
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})

				By("From bpm configs created to variable interpolated state")
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))

				newInstance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, newInstance)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInstance.Status.State).To(Equal(cfd.DeployingState))
			})
		})
	})
})
