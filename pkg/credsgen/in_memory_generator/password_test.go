package inmemorygenerator_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InMemoryGenerator", func() {
	var (
		generator credsgen.Generator = inmemorygenerator.InMemoryGenerator{}
	)

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
