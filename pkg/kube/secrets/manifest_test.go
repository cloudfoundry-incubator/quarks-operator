package secrets_test

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	namespace                = "default"
	deploymentName           = "fake-deployment"
	exampleSourceDescription = "created by a unit-test"
)

func exampleManifest(data string) manifest.Manifest {
	var manifest manifest.Manifest
	err := yaml.Unmarshal([]byte(data), &manifest)
	Expect(err).ToNot(HaveOccurred())
	return manifest
}

var _ = Describe("PersistManifest", func() {
	var (
		kubeClient client.Client
		persister  ManifestPersister
	)

	BeforeEach(func() {
		kubeClient = fake.NewFakeClient()
		persister = NewManifestPersister(kubeClient, namespace, deploymentName)
	})

	Context("when there is no versioned manifest", func() {
		It("should create the first version", func() {
			secretName := fmt.Sprintf("deployment-%s-%d", deploymentName, 1)
			createdSecret, err := persister.PersistManifest(exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(secretName))

			retrievedSecret := &corev1.Secret{}
			err = kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: secretName}, retrievedSecret)
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedSecret).To(BeEquivalentTo(createdSecret))
		})
	})

	Context("when there already is a version of the manifest", func() {
		It("should create a new version", func() {
			createdSecret, err := persister.PersistManifest(exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"}]}`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 1)))

			createdSecret, err = persister.PersistManifest(exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 2)))

			createdSecret, err = persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 3)))
		})
	})

	Context("when the deployment name contains invalid characters", func() {
		It("should fail to create a new version", func() {
			persister = NewManifestPersister(kubeClient, namespace, "InvalidName")
			createdSecret, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).To(HaveOccurred())
			Expect(createdSecret).To(BeNil())
		})
	})

	Context("when the deployment name exceeds a length of 253 characters", func() {
		It("should fail to create a new version", func() {
			persister = NewManifestPersister(kubeClient, namespace, strings.Repeat("foobar", 42))
			createdSecret, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).To(HaveOccurred())
			Expect(createdSecret).To(BeNil())
		})
	})
})

var _ = Describe("DeleteManifest", func() {
	var (
		kubeClient client.Client
		persister  ManifestPersister
	)

	Context("when a manifest with multiple version exists", func() {
		BeforeEach(func() {
			kubeClient = fake.NewFakeClient()
			persister = NewManifestPersister(kubeClient, namespace, deploymentName)

			_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())

			_, err = persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should get rid of all versions of a manifest", func() {
			currentManifestSecrets, err := persister.ListAllVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(currentManifestSecrets)).To(BeIdenticalTo(2))

			err = persister.DeleteManifest()
			Expect(err).ToNot(HaveOccurred())

			currentManifestSecrets, err = persister.ListAllVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(currentManifestSecrets)).To(BeIdenticalTo(0))
		})
	})
})

var _ = Describe("DecorateManifest", func() {
	var (
		kubeClient client.Client
		persister  ManifestPersister
	)

	Context("when there is a manifest with multiple versions", func() {
		BeforeEach(func() {
			kubeClient = fake.NewFakeClient()
			persister = NewManifestPersister(kubeClient, namespace, deploymentName)

			_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())

			_, err = persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should decorate the lastest version with the provided key and value", func() {
			updatedSecret, err := persister.DecorateManifest("foo", "bar")
			Expect(err).ToNot(HaveOccurred())

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

var _ = Describe("ListAllVersions", func() {
	var (
		kubeClient client.Client
		persister  ManifestPersister
	)

	Context("when there is a manifest with multiple versions", func() {
		BeforeEach(func() {
			kubeClient = fake.NewFakeClient()
			persister = NewManifestPersister(kubeClient, namespace, deploymentName)

			for i := 1; i < 10; i++ {
				_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())

				_, err = NewManifestPersister(kubeClient, namespace, "another-deployment").PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should list all versions of a manifest", func() {
			currentManifestSecrets, err := persister.ListAllVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(currentManifestSecrets)).To(BeIdenticalTo(9))
		})
	})
})

var _ = Describe("RetrieveVersion/RetrieveLatestVersion", func() {
	var (
		kubeClient client.Client
		persister  ManifestPersister
	)

	Context("when there is a manifest with multiple versions", func() {
		BeforeEach(func() {
			kubeClient = fake.NewFakeClient()
			persister = NewManifestPersister(kubeClient, namespace, deploymentName)

			for i := 1; i < 10; i++ {
				_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())

				_, err = NewManifestPersister(kubeClient, namespace, "another-deployment").PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should be possible to pick a specific version", func() {
			versionedSecret, err := persister.RetrieveVersion(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(versionedSecret.Name).To(BeIdenticalTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 1)))
		})

		It("should be possible to get the latest version", func() {
			versionedSecret, err := persister.RetrieveLatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(versionedSecret.Name).To(BeIdenticalTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 9)))
		})
	})
})
