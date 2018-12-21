package secrets_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	. "code.cloudfoundry.org/cf-operator/pkg/kube/secrets"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
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
		client    kubernetes.Interface
		persister ManifestPersister
	)

	BeforeEach(func() {
		client = testclient.NewSimpleClientset()
		persister = NewManifestPersister(client, namespace, deploymentName)
	})

	Context("when there is no versioned manifest", func() {
		It("should create the first version", func() {
			createdSecret, err := persister.PersistManifest(exampleManifest(`{"instance-groups":[{"instances":3,"name":"diego"},{"instances":2,"name":"mysql"}]}`), exampleSourceDescription)

			secretName := fmt.Sprintf("deployment-%s-%d", deploymentName, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(secretName))

			retrievedSecret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
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
			persister = NewManifestPersister(client, namespace, "InvalidName")
			createdSecret, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).To(HaveOccurred())
			Expect(createdSecret).To(BeNil())
		})
	})

	Context("when the deployment name exceeds a length of 253 characters", func() {
		It("should fail to create a new version", func() {
			persister = NewManifestPersister(client, namespace, strings.Repeat("foobar", 42))
			createdSecret, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).To(HaveOccurred())
			Expect(createdSecret).To(BeNil())
		})
	})
})

var _ = Describe("DeleteManifest", func() {
	var (
		client    kubernetes.Interface
		persister ManifestPersister
	)

	Context("when a manifest with multiple version exists", func() {
		BeforeEach(func() {
			client = testclient.NewSimpleClientset()
			persister = NewManifestPersister(client, namespace, deploymentName)

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
		client    kubernetes.Interface
		persister ManifestPersister
	)

	Context("when there is a manifest with multiple versions", func() {
		BeforeEach(func() {
			client = testclient.NewSimpleClientset()
			persister = NewManifestPersister(client, namespace, deploymentName)

			_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())

			_, err = persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should decorate the lastest version with the provided key and value", func() {
			secret, err := persister.RetrieveLatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(secret.Name).To(BeIdenticalTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 2)))
			Expect(secret.GetLabels()).To(BeEquivalentTo(map[string]string{
				"deployment-name":    deploymentName,
				"version":            "2",
				"source-description": exampleSourceDescription,
			}))

			updatedSecret, err := persister.DecorateManifest("foo", "bar")
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedSecret.GetLabels()).To(BeEquivalentTo(map[string]string{
				"deployment-name":    deploymentName,
				"version":            "2",
				"source-description": exampleSourceDescription,
				"foo":                "bar",
			}))
		})
	})
})

var _ = Describe("ListAllVersions", func() {
	var (
		client    kubernetes.Interface
		persister ManifestPersister
	)

	Context("when there is a manifest with multiple versions", func() {
		BeforeEach(func() {
			client = testclient.NewSimpleClientset()
			persister = NewManifestPersister(client, namespace, deploymentName)

			for i := 1; i < 10; i++ {
				_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())

				_, err = NewManifestPersister(client, namespace, "another-deployment").PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
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
		client    kubernetes.Interface
		persister ManifestPersister
	)

	Context("when there is a manifest with multiple versions", func() {
		BeforeEach(func() {
			client = testclient.NewSimpleClientset()
			persister = NewManifestPersister(client, namespace, deploymentName)

			for i := 1; i < 10; i++ {
				_, err := persister.PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
				Expect(err).ToNot(HaveOccurred())

				_, err = NewManifestPersister(client, namespace, "another-deployment").PersistManifest(exampleManifest(`instance-groups: []`), exampleSourceDescription)
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
