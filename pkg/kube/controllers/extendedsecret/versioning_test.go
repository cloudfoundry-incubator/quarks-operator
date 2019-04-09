package extendedsecret_test

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

	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionedSecretStore", func() {
	var (
		namespace                string
		secretName               string
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
		secretName = "fake-deployment"
		exampleSourceDescription = "created by a unit-test"
		secretLabels = map[string]string{
			"deployment-name": secretName,
		}

		secretV1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName + "-v1",
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
				Name:      secretName + "-v2",
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
				Name:      secretName + "-v4",
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

	Describe("Create", func() {
		Context("when there is no versioned manifest", func() {
			It("should create the first version", func() {
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						secret := object.(*corev1.Secret)
						Expect(secret.GetName()).To(Equal(fmt.Sprintf("%s-v%d", secretName, 1)))
						return nil
					}
					return nil
				})

				err := store.Create(ctx, namespace, secretName, map[string]string{"manifest": `{"instance_groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`}, secretLabels, exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when there already is a version of the manifest", func() {
			It("should create a new version", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						secretV1.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						secret := object.(*corev1.Secret)
						Expect(secret.GetName()).To(Equal(fmt.Sprintf("%s-v%d", secretName, 1)))
						return nil
					}
					return nil
				})

				err := store.Create(ctx, namespace, secretName, map[string]string{"manifest": `{"instance_groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`}, secretLabels, exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the deployment name exceeds a length of 253 characters", func() {
			It("should fail to create a new version", func() {
				store = NewVersionedSecretStore(client)
				err := store.Create(ctx, namespace, strings.Repeat("foobar", 42), map[string]string{"manifest": `{"instance_groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`}, secretLabels, exampleSourceDescription)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Delete", func() {
		Context("when a manifest with multiple version exists", func() {
			It("should get rid of all versions of a manifest", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1}
						secrets.DeepCopyInto(object.(*corev1.SecretList))
						return nil
					}

					return nil
				})

				client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOptionFunc) error {
					switch object.(type) {
					case *corev1.Secret:
						secret := object.(*corev1.Secret)
						Expect(secret.GetName()).To(Equal(fmt.Sprintf("%s-v%d", secretName, 1)))
						return nil
					}
					return nil
				})

				err := store.Delete(ctx, namespace, secretName, secretLabels)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Decorate", func() {
		Context("when there is a manifest with multiple versions", func() {
			It("should decorate the latest version with the provided key and value", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1}
						secrets.DeepCopyInto(object.(*corev1.SecretList))
						return nil
					}

					return nil
				})

				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						secretV1.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.UpdateCalls(func(_ context.Context, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						secret := object.(*corev1.Secret)
						Expect(secret.Labels).To(HaveKeyWithValue("foo", "bar"))
						return nil
					}

					return nil
				})

				err := store.Decorate(ctx, namespace, secretName, secretLabels, "foo", "bar")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("List", func() {
		Context("when there is a manifest with multiple versions", func() {
			It("should list all versions of a manifest", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1, *secretV2}
						secrets.DeepCopyInto(object.(*corev1.SecretList))
						return nil
					}

					return nil
				})

				currentManifestSecrets, err := store.List(ctx, namespace, secretName, secretLabels)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(currentManifestSecrets)).To(BeIdenticalTo(2))
			})
		})
	})

	Describe("Find/Latest", func() {
		Context("when there is a manifest with multiple versions", func() {
			BeforeEach(func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1, *secretV4}
						secrets.DeepCopyInto(object.(*corev1.SecretList))
						return nil
					}

					return nil
				})

				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						if nn.Name == secretName+"-v1" {
							secretV1.DeepCopyInto(object.(*corev1.Secret))
						} else if nn.Name == secretName+"-v4" {
							secretV4.DeepCopyInto(object.(*corev1.Secret))
						}

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
			})

			It("should be possible to get the latest version", func() {
				secret, err := store.Latest(ctx, namespace, secretName, secretLabels)
				Expect(err).ToNot(HaveOccurred())
				Expect(secret.Name).To(Equal(fmt.Sprintf("%s-v%d", secretName, 4)))
			})
		})
	})

	Describe("Get", func() {
		It("should retrieve the secret for specified version", func() {
			client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object.(type) {
				case *corev1.Secret:
					if nn.Name == secretName+"-v1" {
						secretV1.DeepCopyInto(object.(*corev1.Secret))
					} else if nn.Name == secretName+"-v4" {
						secretV4.DeepCopyInto(object.(*corev1.Secret))
					}

					return nil
				}

				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})

			secret, err := store.Get(ctx, namespace, secretName, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret.Name).To(Equal(fmt.Sprintf("%s-v%d", secretName, 1)))
		})
	})

	Describe("VersionCount", func() {
		Context("when manifest versions exist", func() {
			It("should count the number of versions", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1}
						secrets.DeepCopyInto(object.(*corev1.SecretList))
						return nil
					}

					return nil
				})

				n, err := store.VersionCount(ctx, namespace, secretName, secretLabels)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(1))
			})
		})
		Context("when manifest versions exist", func() {
			It("should count the number of versions", func() {
				client.ListCalls(func(context context.Context, opts *crc.ListOptions, object runtime.Object) error {
					switch object.(type) {
					case *corev1.SecretList:
						secrets := &corev1.SecretList{}

						secrets.Items = []corev1.Secret{*secretV1, *secretV2}
						secrets.DeepCopyInto(object.(*corev1.SecretList))
						return nil
					}

					return nil
				})

				n, err := store.VersionCount(ctx, namespace, secretName, secretLabels)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(2))
			})
		})
	})
})
