package manifest_test

import (
	"fmt"

	fissile "code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	ipl "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/interpolator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	var (
		resolver bdm.Resolver
		client   client.Client
	)

	BeforeEach(func() {
		client = fakeClient.NewFakeClient(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Data: map[string]string{"manifest": "---"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "populated-foo",
					Namespace: "default",
				},
				Data: map[string]string{"manifest": `
instance-groups:
- name: diego
  instances: 3
- name: mysql
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Data: map[string]string{"ops": ""},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "populatedbaz",
					Namespace: "default",
				},
				Data: map[string]string{"ops": `
- type: replace
  path: /instance-groups/name=diego?/instances
  value: 4
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "removeops",
					Namespace: "default",
				},
				Data: map[string]string{"ops": `
- type: remove
  path: /instance-groups
`},
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
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid_ops",
					Namespace: "default",
				},
				Data: map[string]string{"ops": `
- type: invalid-ops
   path: /name
   value: new-deployment
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing_variables",
					Namespace: "default",
				},
				Data: map[string]string{"ops": `
- type: replace
  path: /instance-groups/name=niego/instances
  value: 5
`},
			},
		)
		resolver = bdm.NewResolver(client, ipl.NewInterpolator())
	})

	Describe("ResolveCRD", func() {
		It("works for valid CRs", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "populated-foo"}
			manifest, err := resolver.ResolveCRD(spec, "default")
			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(2))
		})

		It("works for valid CRs containing ops that replace", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "populated-foo", OpsRef: "populatedbaz"}
			manifest, err := resolver.ResolveCRD(spec, "default")
			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(manifest.InstanceGroups[0].Instances).To(Equal(4))
		})

		It("works for valid CRs containing ops that remove", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "populated-foo", OpsRef: "removeops"}
			manifest, err := resolver.ResolveCRD(spec, "default")
			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(0))
		})

		It("throws an error if the CR can not be found", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "bar"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Failed to retrieve configmap '%s/%s' via client.Get", "default", "bar")))
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

		It("throws an error if ops configmap can not be found", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "foo", OpsRef: "boo"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Failed to retrieve configmap '%s/%s' via client.Get", "default", "boo")))
		})

		It("throws an error if ops configmap can not be found", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "foo", OpsRef: "missing_key"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("configmap doesn't contain ops key"))
		})

		It("throws an error if build invalid ops", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "foo", OpsRef: "invalid_ops"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to build ops"))
		})

		It("throws an error if interpolate missing variables into a manifest", func() {
			spec := fissile.BOSHDeploymentSpec{ManifestRef: "populated-foo", OpsRef: "missing_variables"}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to interpolate"))
		})
	})
})
