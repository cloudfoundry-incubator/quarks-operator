package converter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("kube converter", func() {
	var (
		m   *manifest.Manifest
		env testing.Catalog
		err error
	)

	Describe("Variables", func() {
		BeforeEach(func() {
			m, err = env.DefaultBOSHManifest()
			Expect(err).NotTo(HaveOccurred())
			format.TruncatedDiff = false
		})

		act := func() ([]esv1.ExtendedSecret, error) {
			kubeConverter := converter.NewKubeConverter("foo")
			return kubeConverter.Variables(m.Name, m.Variables)
		}

		Context("converting variables", func() {
			It("sanitizes secret names", func() {
				m.Name = "-abc_123.?!\"§$&/()=?"
				m.Variables[0].Name = "def-456.?!\"§$&/()=?-"

				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables[0].Name).To(Equal("abc-123.var-def-456"))
			})

			It("trims secret names to 63 characters", func() {
				m.Name = "foo"
				m.Variables[0].Name = "this-is-waaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay-too-long"

				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables[0].Name).To(Equal("foo.var-this-is-waaaaaaaaaaaaaa5bffdb0302ac051d11f52d2606254a5f"))
			})

			It("converts password variables", func() {
				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(variables)).To(Equal(1))

				var1 := variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-adminpass"))
				Expect(var1.Spec.Type).To(Equal(esv1.Password))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-adminpass"))
			})

			It("converts rsa key variables", func() {
				m.Variables[0] = manifest.Variable{
					Name: "adminkey",
					Type: "rsa",
				}
				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables).To(HaveLen(1))

				var1 := variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-adminkey"))
				Expect(var1.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))
				Expect(var1.Spec.Type).To(Equal(esv1.RSAKey))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-adminkey"))
			})

			It("converts ssh key variables", func() {
				m.Variables[0] = manifest.Variable{
					Name: "adminkey",
					Type: "ssh",
				}
				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables).To(HaveLen(1))

				var1 := variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-adminkey"))
				Expect(var1.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))
				Expect(var1.Spec.Type).To(Equal(esv1.SSHKey))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-adminkey"))
			})

			It("raises an error when the options are missing for a certificate variable", func() {
				m.Variables[0] = manifest.Variable{
					Name: "foo-cert",
					Type: "certificate",
				}
				_, err := act()
				Expect(err).To(HaveOccurred())
			})

			It("converts certificate variables", func() {
				m.Variables[0] = manifest.Variable{
					Name: "foo-cert",
					Type: "certificate",
					Options: &manifest.VariableOptions{
						CommonName:       "example.com",
						AlternativeNames: []string{"foo.com", "bar.com"},
						IsCA:             true,
						CA:               "theca",
						ExtendedKeyUsage: []manifest.AuthType{manifest.ClientAuth},
					},
				}
				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables).To(HaveLen(1))

				var1 := variables[0]
				Expect(var1.Name).To(Equal("foo-deployment.var-foo-cert"))
				Expect(var1.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))
				Expect(var1.Spec.Type).To(Equal(esv1.Certificate))
				Expect(var1.Spec.SecretName).To(Equal("foo-deployment.var-foo-cert"))
				request := var1.Spec.Request.CertificateRequest
				Expect(request.CommonName).To(Equal("example.com"))
				Expect(request.AlternativeNames).To(Equal([]string{"foo.com", "bar.com"}))
				Expect(request.IsCA).To(Equal(true))
				Expect(request.CARef.Name).To(Equal("foo-deployment.var-theca"))
				Expect(request.CARef.Key).To(Equal("certificate"))
			})
		})

	})
})
