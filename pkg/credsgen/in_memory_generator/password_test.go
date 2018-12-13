package inmemorygenerator_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	"code.cloudfoundry.org/cf-operator/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InMemoryGenerator", func() {
	var (
		generator credsgen.Generator
	)

	BeforeEach(func() {
		_, log := util.NewTestLogger()
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
