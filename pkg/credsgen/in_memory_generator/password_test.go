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

	Describe("GeneratePassword", func() {
		It("has a default length", func() {
			password := generator.GeneratePassword("foo", credsgen.PasswordGenerationRequest{})

			Expect(len(password)).To(Equal(credsgen.DefaultPasswordLength))
		})

		It("considers custom lengths", func() {
			password := generator.GeneratePassword("foo", credsgen.PasswordGenerationRequest{Length: 10})

			Expect(len(password)).To(Equal(10))
		})
	})
})
