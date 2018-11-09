package manifest_test

import (
	fissile "code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	var (
		resolver bdm.Resolver
		client   client.Client
	)

	BeforeEach(func() {
		client = fake.NewFakeClient(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Data: map[string]string{"manifest": "---"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid_yaml",
					Namespace: "default",
				},
				Data: map[string]string{"manifest": "!yaml"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing_key",
					Namespace: "default",
				},
				Data: map[string]string{},
			},
		)
		resolver = bdm.NewResolver(client)
	})

	Describe("ResolveCRD", func() {
		It("works for valid CRs", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "foo"}
			manifest, err := resolver.ResolveCRD(spec, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(0))
		})

		It("throws an error if the CR can not be found", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "bar"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("configmaps \"bar\" not found"))
		})

		It("throws an error if the CR is empty", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "missing_key"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("configmap doesn't contain manifest key"))
		})

		It("throws an error on invalid yaml", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "invalid_yaml"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("yaml: unmarshal errors"))
		})
	})
})
