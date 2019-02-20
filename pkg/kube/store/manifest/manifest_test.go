package manifest_test

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"
	yaml "gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	ctx "code.cloudfoundry.org/cf-operator/pkg/kube/util/context"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/store/manifest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	namespace                = "default"
	deploymentName           = "fake-deployment"
	exampleSourceDescription = "created by a unit-test"
)

var _ = Describe("Store", func() {
	var (
		client  crc.Client
		store   Store
		context context.Context
	)

	exampleManifest := func(data string) manifest.Manifest {
		var manifest manifest.Manifest
		err := yaml.Unmarshal([]byte(data), &manifest)
		Expect(err).ToNot(HaveOccurred())
		return manifest
	}

	hasSecret := func(name string) bool {
		secret := &corev1.Secret{}
		err := client.Get(context, crc.ObjectKey{Namespace: namespace, Name: name}, secret)
		return err == nil
	}

	getSecret := func(name string) *corev1.Secret {
		secret := &corev1.Secret{}
		err := client.Get(context, crc.ObjectKey{Namespace: namespace, Name: name}, secret)
		Expect(err).ToNot(HaveOccurred())
		return secret
	}

	secretCount := func() int {
		secrets := &corev1.SecretList{}
		err := client.List(context, crc.InNamespace(namespace), secrets)
		Expect(err).ToNot(HaveOccurred())
		return len(secrets.Items)
	}

	BeforeEach(func() {
		client = fake.NewFakeClient()
		store = NewStore(client, namespace, deploymentName)
		context = ctx.NewBackgroundContext()
	})

	Describe("Save", func() {
		Context("when there is no versioned manifest", func() {
			It("should create the first version", func() {
				err := store.Save(context, exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
				Expect(secretCount()).To(Equal(1))

				name := fmt.Sprintf("deployment-%s-%d", deploymentName, 1)
				Expect(hasSecret(name)).To(BeTrue())
			})
		})

		Context("when there already is a version of the manifest", func() {

			It("should create a new version", func() {
				err := store.Save(context, exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"}]}`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
				name := fmt.Sprintf("deployment-%s-%d", deploymentName, 1)
				Expect(hasSecret(name)).To(BeTrue())

				err = store.Save(context, exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
				name = fmt.Sprintf("deployment-%s-%d", deploymentName, 2)
				Expect(hasSecret(name)).To(BeTrue())

				err = store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
				name = fmt.Sprintf("deployment-%s-%d", deploymentName, 3)
				Expect(hasSecret(name)).To(BeTrue())
			})
		})

		Context("when the deployment name contains invalid characters", func() {
			It("should fail to create a new version", func() {
				store = NewStore(client, namespace, "InvalidName")
				err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).To(HaveOccurred())

				Expect(secretCount()).To(Equal(0))
			})
		})

		Context("when the deployment name exceeds a length of 253 characters", func() {
			It("should fail to create a new version", func() {
				store = NewStore(client, namespace, strings.Repeat("foobar", 42))
				err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).To(HaveOccurred())
				Expect(secretCount()).To(Equal(0))
			})
		})
	})

	Describe("Delete", func() {
		Context("when a manifest with multiple version exists", func() {
			BeforeEach(func() {
				err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())

				err = store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get rid of all versions of a manifest", func() {
				currentManifestSecrets, err := store.List(context)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(currentManifestSecrets)).To(BeIdenticalTo(2))

				err = store.Delete(context)
				Expect(err).ToNot(HaveOccurred())

				currentManifestSecrets, err = store.List(context)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(currentManifestSecrets)).To(BeIdenticalTo(0))
			})
		})
	})

	Describe("Decorate", func() {
		Context("when there is a manifest with multiple versions", func() {
			BeforeEach(func() {
				err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())

				err = store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should decorate the lastest version with the provided key and value", func() {
				err := store.Decorate(context, "foo", "bar")
				Expect(err).ToNot(HaveOccurred())

				name := fmt.Sprintf("deployment-%s-%d", deploymentName, 2)
				updatedSecret := getSecret(name)

				Expect(updatedSecret.GetLabels()).To(BeEquivalentTo(map[string]string{
					"deployment-name": deploymentName,
					"version":         "2",
					"foo":             "bar",
				}))
				Expect(updatedSecret.GetAnnotations()).To(BeEquivalentTo(map[string]string{
					"source-description": exampleSourceDescription,
				}))
			})
		})
	})

	Describe("List", func() {
		Context("when there is a manifest with multiple versions", func() {
			BeforeEach(func() {
				for i := 1; i < 10; i++ {
					err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
					Expect(err).ToNot(HaveOccurred())

					err = NewStore(client, namespace, "another-deployment").Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should list all versions of a manifest", func() {
				currentManifestSecrets, err := store.List(context)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(currentManifestSecrets)).To(BeIdenticalTo(9))
			})
		})
	})

	Describe("Find/Latest", func() {
		Context("when there is a manifest with multiple versions", func() {
			BeforeEach(func() {
				for i := 1; i < 10; i++ {
					m := exampleManifest(fmt.Sprintf("instance-groups: [{name: ig%d}]", i))
					err := store.Save(context, m, exampleSourceDescription)
					Expect(err).ToNot(HaveOccurred())

					err = NewStore(client, namespace, "another-deployment").Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should be possible to pick a specific version", func() {
				manifest, err := store.Find(context, 1)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.InstanceGroups).To(HaveLen(1))
				Expect(manifest.InstanceGroups[0].Name).To(Equal("ig1"))

				manifest, err = store.Find(context, 4)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.InstanceGroups).To(HaveLen(1))
				Expect(manifest.InstanceGroups[0].Name).To(Equal("ig4"))
			})

			It("should be possible to get the latest version", func() {
				manifest, err := store.Latest(context)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.InstanceGroups).To(HaveLen(1))
				Expect(manifest.InstanceGroups[0].Name).To(Equal("ig9"))
			})
		})
	})

	Describe("RetrieveVersionSecret", func() {
		BeforeEach(func() {
			err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should retrieve the secret for specified version", func() {
			secret, err := store.RetrieveVersionSecret(context, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret.Name).To(Equal(fmt.Sprintf("deployment-%s-%d", deploymentName, 1)))
		})
	})

	Describe("VersionCount", func() {
		Context("when no manifest versions exist", func() {
			It("should return zero", func() {
				n, err := store.VersionCount(context)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(0))
			})
		})

		Context("when manifest versions exist", func() {

			BeforeEach(func() {
				err := store.Save(context, exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should count the number of versions", func() {
				n, err := store.VersionCount(context)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(1))
			})
		})
	})
})
