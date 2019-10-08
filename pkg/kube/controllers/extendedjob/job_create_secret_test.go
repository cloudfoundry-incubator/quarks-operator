package extendedjob_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	ejapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	testversionedclient "code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned/fake"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("PersistOutputInterface", func() {
	var (
		namespace          string
		ejob               *ejapi.ExtendedJob
		job                *batchv1.Job
		pod1               *corev1.Pod
		env                testing.Catalog
		clientSet          *testclient.Clientset
		versionedClientSet *testversionedclient.Clientset
		po1                *extendedjob.PersistOutputInterface
	)

	BeforeEach(func() {
		namespace = "test"
		ejob, job, pod1 = env.DefaultExtendedJobWithSucceededJob("foo")
		clientSet = testclient.NewSimpleClientset()
		versionedClientSet = testversionedclient.NewSimpleClientset()
		po1 = extendedjob.NewPersistOutputInterface(namespace, pod1.Name, clientSet, versionedClientSet, "/tmp/")
	})

	JustBeforeEach(func() {

		// Create necessary kube resources
		_, err := versionedClientSet.ExtendedjobV1alpha1().ExtendedJobs(namespace).Create(ejob)
		Expect(err).NotTo(HaveOccurred())
		_, err = clientSet.BatchV1().Jobs(namespace).Create(job)
		Expect(err).NotTo(HaveOccurred())
		_, err = clientSet.CoreV1().Pods(namespace).Create(pod1)
		Expect(err).NotTo(HaveOccurred())

		// Create output file
		err = os.MkdirAll("/tmp/busybox", os.ModePerm)
		Expect(err).NotTo(HaveOccurred())
		dataJson, err := json.Marshal(map[string]string{
			"hello": "world",
		})
		Expect(err).NotTo(HaveOccurred())
		err = ioutil.WriteFile("/tmp/busybox/output.json", dataJson, 0755)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("With a succeeded Job", func() {
		Context("when output persistence is not configured", func() {
			BeforeEach(func() {
				ejob.Spec.Output = nil
			})

			It("does not persist output", func() {
				err := po1.PersistOutput()
				Expect(err).NotTo(HaveOccurred())
				_, err = clientSet.CoreV1().Secrets(namespace).Get("foo-busybox", metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when output persistence is configured", func() {
			BeforeEach(func() {
				ejob.Spec.Output = &ejapi.Output{
					NamePrefix: "foo-",
					SecretLabels: map[string]string{
						"key": "value",
					},
				}
			})

			It("creates the secret and persists the output and have the configured labels", func() {
				err := po1.PersistOutput()
				Expect(err).NotTo(HaveOccurred())
				secret, _ := clientSet.CoreV1().Secrets(namespace).Get("foo-busybox", metav1.GetOptions{})
				Expect(secret).ShouldNot(BeNil())
				Expect(secret.Labels).Should(Equal(map[string]string{
					"fissile.cloudfoundry.org/container-name": "busybox",
					"key": "value"}))
			})
		})

		Context("when versioned output is enabled", func() {
			BeforeEach(func() {
				ejob.Spec.Output = &ejapi.Output{
					NamePrefix: "foo-",
					SecretLabels: map[string]string{
						"key":        "value",
						"fake-label": "fake-deployment",
					},
					Versioned: true,
				}
			})

			It("creates versioned manifest secret and persists the output", func() {
				err := po1.PersistOutput()
				Expect(err).NotTo(HaveOccurred())
				secret, _ := clientSet.CoreV1().Secrets(namespace).Get("foo-busybox-v1", metav1.GetOptions{})
				Expect(secret).ShouldNot(BeNil())
				Expect(secret.Labels).Should(Equal(map[string]string{
					"fissile.cloudfoundry.org/container-name": "busybox",
					"fake-label":                         "fake-deployment",
					versionedsecretstore.LabelSecretKind: "versionedSecret",
					versionedsecretstore.LabelVersion:    "1",
					"key":                                "value"}))
			})
		})
	})
})
