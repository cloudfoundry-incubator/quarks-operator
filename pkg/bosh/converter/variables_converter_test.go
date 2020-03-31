package converter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("kube converter", func() {
	var (
		deploymentName string
		m              *manifest.Manifest
		env            testing.Catalog
		err            error
	)

	Describe("Variables", func() {
		BeforeEach(func() {
			deploymentName = "foo-deployment"
			m, err = env.DefaultBOSHManifest()
			Expect(err).NotTo(HaveOccurred())
			format.TruncatedDiff = false
		})

		act := func() ([]qsv1a1.QuarksSecret, error) {
			kubeConverter := converter.NewVariablesConverter("foo")
			return kubeConverter.Variables(deploymentName, m.Variables)
		}

		Context("converting variables", func() {
			It("sanitizes secret names", func() {
				deploymentName = "-abc_123.?!\"§$&/()=?"
				m.Variables[0].Name = "def-456.?!\"§$&/()=?-"

				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables[0].Name).To(Equal("var-def-456"))
			})

			It("trims secret names", func() {
				deploymentName = "foo"
				long := "this-is-waaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				m.Variables[0].Name = long + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay-too-long"

				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(variables[0].Name).To(Equal("var-" + long[:216] + "-cd1aedf32e5af6a8401880952515840e"))
			})

			It("converts password variables", func() {
				variables, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(variables)).To(Equal(1))

				var1 := variables[0]
				Expect(var1.Name).To(Equal("var-adminpass"))
				Expect(var1.Spec.Type).To(Equal(qsv1a1.Password))
				Expect(var1.Spec.SecretName).To(Equal("var-adminpass"))
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
				Expect(var1.Name).To(Equal("var-adminkey"))
				Expect(var1.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentName, deploymentName))
				Expect(var1.Spec.Type).To(Equal(qsv1a1.RSAKey))
				Expect(var1.Spec.SecretName).To(Equal("var-adminkey"))
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
				Expect(var1.Name).To(Equal("var-adminkey"))
				Expect(var1.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentName, deploymentName))
				Expect(var1.Spec.Type).To(Equal(qsv1a1.SSHKey))
				Expect(var1.Spec.SecretName).To(Equal("var-adminkey"))
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
				Expect(var1.Name).To(Equal("var-foo-cert"))
				Expect(var1.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentName, deploymentName))
				Expect(var1.Spec.Type).To(Equal(qsv1a1.Certificate))
				Expect(var1.Spec.SecretName).To(Equal("var-foo-cert"))
				request := var1.Spec.Request.CertificateRequest
				Expect(request.CommonName).To(Equal("example.com"))
				Expect(request.AlternativeNames).To(Equal([]string{"foo.com", "bar.com"}))
				Expect(request.IsCA).To(Equal(true))
				Expect(request.CARef.Name).To(Equal("var-theca"))
				Expect(request.CARef.Key).To(Equal("certificate"))
			})
		})

	})
})
