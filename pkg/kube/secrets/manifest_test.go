package secrets_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/secrets"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

// Note: Assumption is that with desired manifests, a fully rendered/evaluated BOSH deployment manifest is meant.

var _ = Describe(`Versioning and persistence for "desired manifests"`, func() {

	exampleManifest := func(data []byte) manifest.Manifest {
		var manifest manifest.Manifest
		err := yaml.Unmarshal(data, &manifest)
		Expect(err).ToNot(HaveOccurred())

		return manifest
	}

	var namespace = "default"
	var deploymentName = "fake-deployment"

	Context("Creating a desired manifest secret", func() {
		var client kubernetes.Interface
		var persister *ManifestPersisterImpl
		var secretName string

		BeforeEach(func() {
			client = testclient.NewSimpleClientset()
			persister = NewManifestPersister(client, namespace, deploymentName)
		})

		It("should create a secret when there is no versioned secret of the manifest", func() {
			createdSecret, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`---
instance-groups:
- name: diego
  instances: 3  
- name: mysql
  instances: 2
`)))

			secretName = fmt.Sprintf("deployment-%s-%d", deploymentName, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(secretName))

			retrievedSecret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedSecret).To(BeEquivalentTo(createdSecret))
		})

		It("should create a new versioned secret when already a version exists", func() {
			createdSecret, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`---
instance-groups:
- name: diego
  instances: 3
`)))
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 1)))

			createdSecret, err = persister.CreateDesiredManifestSecret(exampleManifest([]byte(`---
instance-groups:
- name: diego
  instances: 3
- name: mysql
  instances: 2
`)))
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 2)))

			createdSecret, err = persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSecret.Name).To(BeEquivalentTo(fmt.Sprintf("deployment-%s-%d", deploymentName, 3)))
		})

		It("should fail to create a secret, when the deployment name contains invalid characters", func() {
			persister = NewManifestPersister(client, namespace, "InvalidName")
			createdSecret, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).To(HaveOccurred())
			Expect(createdSecret).To(BeNil())
		})

		It("should fail to create a secret, when the deployment name exceeds 253 chars length", func() {
			persister = NewManifestPersister(client, namespace, strings.Repeat("foobar", 42))
			createdSecret, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).To(HaveOccurred())
			Expect(createdSecret).To(BeNil())
		})
	})

	Context("Listing secrets", func() {
		var client kubernetes.Interface
		var persister *ManifestPersisterImpl

		BeforeEach(func() {
			client = testclient.NewSimpleClientset()
			persister = NewManifestPersister(client, namespace, deploymentName)
		})

		It("should give you all secrets of a deployment", func() {
			for i := 1; i < 10; i++ {
				_, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
				Expect(err).ToNot(HaveOccurred())

				_, err = NewManifestPersister(client, namespace, "another-deployment").
					CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
				Expect(err).ToNot(HaveOccurred())
			}
			currentManifestSecrets, err := persister.ListAllVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(currentManifestSecrets)).To(BeIdenticalTo(9))
		})
	})

	Context("Retrieving secret versions", func() {
		var client kubernetes.Interface
		var persister *ManifestPersisterImpl

		BeforeEach(func() {
			client = testclient.NewSimpleClientset()

			persister = NewManifestPersister(client, namespace, "cf")
			_, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).ToNot(HaveOccurred())
			_, err = persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should give you an specific secret version", func() {
			versionedSecret, err := persister.RetrieveVersion(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(versionedSecret.Name).To(BeIdenticalTo(fmt.Sprintf("deployment-%s-%d", "cf", 1)))
		})

		It("should give you the latest secret version", func() {
			versionedSecret, err := persister.RetrieveLatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(versionedSecret.Name).To(BeIdenticalTo(fmt.Sprintf("deployment-%s-%d", "cf", 2)))
		})
	})

	Context("Delete all versions of a secret", func() {
		var client kubernetes.Interface
		var persister *ManifestPersisterImpl

		BeforeEach(func() {
			client = testclient.NewSimpleClientset()

			persister = NewManifestPersister(client, namespace, "cf")
			_, err := persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).ToNot(HaveOccurred())
			_, err = persister.CreateDesiredManifestSecret(exampleManifest([]byte(`instance-groups: []`)))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should get rid of all versions of a secret", func() {
			currentManifestSecrets, err := persister.ListAllVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(currentManifestSecrets)).To(BeIdenticalTo(2))

			err = persister.DeleteManifestSecrets()
			Expect(err).ToNot(HaveOccurred())

			currentManifestSecrets, err = persister.ListAllVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(currentManifestSecrets)).To(BeIdenticalTo(0))
		})
	})
})
