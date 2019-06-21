package versionedsecretstore_test

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	. "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	"code.cloudfoundry.org/cf-operator/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionedSecretStore", func() {
	var (
		namespace                string
		secretNamePrefix         string
		exampleSourceDescription string
		secretLabels             map[string]string
		secretV1                 *corev1.Secret
		secretV2                 *corev1.Secret
		secretV4                 *corev1.Secret

		client *cfakes.FakeClient
		store  VersionedSecretStore
		ctx    context.Context
	)

	BeforeEach(func() {
		namespace = "default"
		secretNamePrefix = "fake-deployment"
		exampleSourceDescription = "created by a unit-test"
		secretLabels = map[string]string{
			"deployment-name": secretNamePrefix,
		}

		secretV1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretNamePrefix + "-v1",
				Namespace: "default",
				UID:       "",
				Labels: map[string]string{
					LabelSecretKind: "versionedSecret",
					LabelVersion:    "1",
				},
			},
			Data: map[string][]byte{
				"manifest": []byte(`instance_groups:
- azs: []
  instances: 2
  name: fake-instance
name: fake-deployment-v1
`),
			},
		}
		secretV2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretNamePrefix + "-v2",
				Namespace: "default",
				UID:       "",
				Labels: map[string]string{
					LabelSecretKind: "versionedSecret",
					LabelVersion:    "2",
				},
			},
			Data: map[string][]byte{
				"manifest": []byte(`instance_groups:
- azs: []
  instances: 2
  name: fake-instance
name: fake-deployment-v4
`),
			},
		}
		secretV4 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretNamePrefix + "-v4",
				Namespace: "default",
				UID:       "",
				Labels: map[string]string{
					LabelSecretKind: "versionedSecret",
					LabelVersion:    "4",
				},
			},
			Data: map[string][]byte{
				"manifest": []byte(`instance_groups:
- azs: []
  instances: 2
  name: fake-instance
name: fake-deployment-v4
`),
			},
		}
	})

	BeforeEach(func() {
		client = &cfakes.FakeClient{}
		store = NewVersionedSecretStore(client)
		ctx = testing.NewContext()
	})

	Describe("SetSecretReferences", func() {
		Context("when there is an extendedStatefulSet", func() {
			var (
				podSpec *corev1.PodSpec
			)

			BeforeEach(func() {
				podSpec = &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name: "SECRET_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretNamePrefix + "-v1",
											},
										},
									},
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretNamePrefix + "-v1",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "secret-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNamePrefix + "-v1",
								},
							},
						},
					},
				}
			})

			It("should replace references with a new versioned secret if there is one version", func() {
				client.ListCalls(func(_ context.Context, options *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						object.Items = []corev1.Secret{
							*secretV1,
						}
						return nil
					}

					return nil
				})

				err := store.SetSecretReferences(ctx, namespace, podSpec)
				Expect(err).ToNot(HaveOccurred())

				// Only one secret reference and it latest version
				_, secretsInSpec := owner.GetConfigNamesFromSpec(*podSpec)
				Expect(len(secretsInSpec)).To(Equal(1))
				Expect(secretsInSpec).To(HaveKey(secretV1.Name))
			})

			It("should replace references with latest version if there is two versions", func() {
				podSpec.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name = secretV1.GetName()
				podSpec.Containers[0].EnvFrom[0].SecretRef.Name = secretV1.GetName()
				podSpec.Volumes[0].VolumeSource.Secret.SecretName = secretV1.GetName()

				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						if nn.Name == secretV1.GetName() {
							secretV1.DeepCopyInto(object)
							return nil
						}
						if nn.Name == secretV2.GetName() {
							secretV2.DeepCopyInto(object)
							return nil
						}
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				client.ListCalls(func(_ context.Context, options *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						object.Items = []corev1.Secret{
							*secretV1,
							*secretV2,
						}
						return nil
					}

					return nil
				})

				err := store.SetSecretReferences(ctx, namespace, podSpec)
				Expect(err).ToNot(HaveOccurred())

				// Only one secret reference and it latest version
				_, secretsInSpec := owner.GetConfigNamesFromSpec(*podSpec)
				Expect(len(secretsInSpec)).To(Equal(1))
				Expect(secretsInSpec).To(HaveKey(secretV2.Name))
			})

			It("should return error if it fails in getting latest versioned secret", func() {
				podSpec.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name = secretV1.GetName()
				podSpec.Containers[0].EnvFrom[0].SecretRef.Name = secretV1.GetName()
				podSpec.Volumes[0].VolumeSource.Secret.SecretName = secretV1.GetName()

				client.ListCalls(func(_ context.Context, options *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						object.Items = []corev1.Secret{
							*secretV1,
							*secretV2,
						}
						return apierrors.NewBadRequest("fake-error")
					}

					return nil
				})

				err := store.SetSecretReferences(ctx, namespace, podSpec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get latest versioned secret %s in namespace %s", secretNamePrefix, namespace))
			})
		})

	})

	Describe("Create", func() {
		Context("when there is no versioned manifest", func() {
			It("should create the first version", func() {
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						Expect(object.GetName()).To(Equal(fmt.Sprintf("%s-v%d", secretNamePrefix, 1)))
						return nil
					}
					return nil
				})

				err := store.Create(
					ctx,
					namespace,
					"some-owner",
					types.UID("d3d423b7-a57f-43b0-8305-79d484154e4f"),
					secretNamePrefix,
					map[string]string{
						"manifest": `{"instance_groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`,
					},
					secretLabels,
					exampleSourceDescription,
				)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when there already is a version of the manifest", func() {
			It("should create a new version", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						secretV1.DeepCopyInto(object)
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						Expect(object.GetName()).To(Equal(fmt.Sprintf("%s-v%d", secretNamePrefix, 1)))
						return nil
					}
					return nil
				})

				err := store.Create(
					ctx,
					namespace,
					"some-owner",
					types.UID("d3d423b7-a57f-43b0-8305-79d484154e4f"),
					secretNamePrefix,
					map[string]string{
						"manifest": `{"instance_groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`,
					},
					secretLabels,
					exampleSourceDescription,
				)
				Expect(err).ToNot(HaveOccurred())
			})
		}) 

		Context("when the deployment name exceeds a length of 253 characters", func() {
			It("should fail to create a new version", func() {
				store = NewVersionedSecretStore(client)
				err := store.Create(
					ctx,
					namespace,
					"some-owner",
					types.UID("d3d423b7-a57f-43b0-8305-79d484154e4f"),
					strings.Repeat("foobar", 42),
					map[string]string{
						"manifest": `{"instance_groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`,
					}, 
					secretLabels,
					exampleSourceDescription,
				)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Delete", func() {
		Context("when a manifest with multiple version exists", func() {
			It("should get rid of all versions of a manifest", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1}
						secrets.DeepCopyInto(object)
						return nil
					}

					return nil
				})

				client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOptionFunc) error {
					switch object := object.(type) {
					case *corev1.Secret:
						Expect(object.GetName()).To(Equal(fmt.Sprintf("%s-v%d", secretNamePrefix, 1)))
						return nil
					}
					return nil
				})

				err := store.Delete(ctx, namespace, secretNamePrefix)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Decorate", func() {
		Context("when there is a manifest with multiple versions", func() {
			It("should decorate the latest version with the provided key and value", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1}
						secrets.DeepCopyInto(object)
						return nil
					}

					return nil
				})

				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						secretV1.DeepCopyInto(object)
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.UpdateCalls(func(_ context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						Expect(object.Labels).To(HaveKeyWithValue("foo", "bar"))
						return nil
					}

					return nil
				})

				err := store.Decorate(ctx, namespace, secretNamePrefix, "foo", "bar")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("List", func() {
		Context("when there is a manifest with multiple versions", func() {
			It("should list all versions of a manifest", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1, *secretV2}
						secrets.DeepCopyInto(object)
						return nil
					}

					return nil
				})

				currentManifestSecrets, err := store.List(ctx, namespace, secretNamePrefix)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(currentManifestSecrets)).To(BeIdenticalTo(2))
			})
		})
	})

	Describe("Find/Latest", func() {
		Context("when there is a manifest with multiple versions", func() {
			BeforeEach(func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1, *secretV4}
						secrets.DeepCopyInto(object)
						return nil
					}

					return nil
				})

				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						if nn.Name == secretNamePrefix+"-v1" {
							secretV1.DeepCopyInto(object)
						} else if nn.Name == secretNamePrefix+"-v4" {
							secretV4.DeepCopyInto(object)
						}

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
			})

			It("should be possible to get the latest version", func() {
				secret, err := store.Latest(ctx, namespace, secretNamePrefix)
				Expect(err).ToNot(HaveOccurred())
				Expect(secret.Name).To(Equal(fmt.Sprintf("%s-v%d", secretNamePrefix, 4)))
			})
		})
	})

	Describe("Get", func() {
		It("should retrieve the secret for specified version", func() {
			client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *corev1.Secret:
					if nn.Name == secretNamePrefix+"-v1" {
						secretV1.DeepCopyInto(object)
					} else if nn.Name == secretNamePrefix+"-v4" {
						secretV4.DeepCopyInto(object)
					}

					return nil
				}

				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})

			secret, err := store.Get(ctx, namespace, secretNamePrefix, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret.Name).To(Equal(fmt.Sprintf("%s-v%d", secretNamePrefix, 1)))
		})
	})

	Describe("VersionCount", func() {
		Context("when manifest versions exist", func() {
			It("should count the number of versions", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1}
						secrets.DeepCopyInto(object)
						return nil
					}

					return nil
				})

				n, err := store.VersionCount(ctx, namespace, secretNamePrefix)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(1))
			})
		})
		Context("when manifest versions exist", func() {
			It("should count the number of versions", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1, *secretV2}
						secrets.DeepCopyInto(object)
						return nil
					}

					return nil
				})

				n, err := store.VersionCount(ctx, namespace, secretNamePrefix)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(2))
			})
		})
	})
})
