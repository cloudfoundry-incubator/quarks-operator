package manifest_test

import (
	"fmt"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeploymentcontroller/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	var (
		resolver     bdm.Resolver
		client       client.Client
		interpolator *fakes.FakeInterpolator
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
					Name:      "baz",
					Namespace: "default",
				},
				Data: map[string]string{"ops": "---"},
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
   path: /missing_key
   value: desired_value
`},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ops-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{"ops": []byte("LS0t")},
			},
		)
		interpolator = &fakes.FakeInterpolator{}
		resolver = bdm.NewResolver(client, interpolator)
	})

	Describe("ResolveCRD", func() {
		It("works for valid CRs", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
			}
			manifest, err := resolver.ResolveCRD(spec, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(0))
		})

		It("works for valid CRs containing ops", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "configMap",
						Ref:  "baz",
					},
				},
			}
			manifest, err := resolver.ResolveCRD(spec, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(0))
		})

		It("works for valid CRs containing multi ops", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "configMap",
						Ref:  "baz",
					},
					{
						Type: "secret",
						Ref:  "ops-secret",
					},
				},
			}
			manifest, err := resolver.ResolveCRD(spec, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(0))
		})

		It("throws an error if the CR can not be found", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "bar",
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Failed to retrieve configmap '%s/%s' via client.Get", "default", "bar")))
		})

		It("throws an error if the CR is empty", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "missing_key",
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("configmap doesn't contain manifest key"))
		})

		It("throws an error on invalid yaml", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "invalid_yaml",
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("yaml: unmarshal errors"))
		})

		It("throws an error if containing unsupported manifest type", func() {
			interpolator.InterpolateReturns(nil, errors.New("fake-error"))
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "unsupported_type",
					Ref:  "foo",
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unrecognized manifest type"))
		})

		It("throws an error if ops configMap can not be found", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "configMap",
						Ref:  "boo",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Failed to retrieve config map '%s/%s' via client.Get", "default", "boo")))
		})

		It("throws an error if ops configMap can not be found", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "configMap",
						Ref:  "missing_key",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("config map doesn't contain ops key"))
		})

		It("throws an error if build invalid ops", func() {
			interpolator.BuildOpsReturns(errors.New("fake-error"))

			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "configMap",
						Ref:  "invalid_ops",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to build ops"))
		})

		It("throws an error if interpolate missing variables into a manifest", func() {
			interpolator.InterpolateReturns(nil, errors.New("fake-error"))
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "configMap",
						Ref:  "missing_variables",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to interpolate"))
		})

		It("throws an error if containing unsupported ops type", func() {
			interpolator.InterpolateReturns(nil, errors.New("fake-error"))
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "unsupported_type",
						Ref:  "variables",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unrecognized ops-ref type"))
		})

		It("throws an error if ops configMap can not be found when contains multi-refs", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "secret",
						Ref:  "ops-secret",
					},
					{
						Type: "configMap",
						Ref:  "missing_key",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to build ops from config map"))
		})

		It("throws an error if ops secret can not be found when contains multi-refs", func() {
			spec := bdc.BOSHDeploymentSpec{
				Manifest: bdc.Manifest{
					Type: "configMap",
					Ref:  "foo",
				},
				Ops: []bdc.Ops{
					{
						Type: "secret",
						Ref:  "missing_key",
					},
					{
						Type: "configMap",
						Ref:  "baz",
					},
				},
			}
			_, err := resolver.ResolveCRD(spec, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to build ops from secret"))
		})
	})
})
