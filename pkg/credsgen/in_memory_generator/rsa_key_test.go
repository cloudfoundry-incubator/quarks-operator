package inmemorygenerator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-operator/pkg/credsgen"
	inmemorygenerator "code.cloudfoundry.org/quarks-operator/pkg/credsgen/in_memory_generator"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("InMemoryGenerator", func() {
	var (
		generator credsgen.Generator
	)

	BeforeEach(func() {
		_, log := helper.NewTestLogger()
		generator = inmemorygenerator.NewInMemoryGenerator(log)
	})

	Describe("GenerateRSAKey", func() {
		It("generates an RSA key", func() {
			key, err := generator.GenerateRSAKey("foo")

			Expect(err).ToNot(HaveOccurred())
			Expect(key.PrivateKey).To(ContainSubstring("BEGIN RSA PRIVATE KEY"))
			Expect(key.PublicKey).To(ContainSubstring("BEGIN PUBLIC KEY"))
		})
	})
})
