package inmemorygenerator_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	inmemorygenerator "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	"code.cloudfoundry.org/cf-operator/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InMemoryGenerator", func() {
	var (
		defaultGenerator credsgen.Generator
	)

	BeforeEach(func() {
		_, log := util.NewTestLogger()
		defaultGenerator = inmemorygenerator.NewInMemoryGenerator(log)
	})

	Describe("NewInMemoryGenerator", func() {
		Context("object defaults", func() {
			It("succeds if the default type is inmemorygenerator.InMemoryGenerator", func() {
				t, ok := defaultGenerator.(*inmemorygenerator.InMemoryGenerator)
				Expect(ok).To(BeTrue())
				Expect(t).To(Equal(defaultGenerator))
			})

			It("succeds if the default generator is 4096 bits", func() {
				Expect(defaultGenerator.(*inmemorygenerator.InMemoryGenerator).Bits).To(Equal(4096))
			})

			It("succeds if the default generator is rsa", func() {
				Expect(defaultGenerator.(*inmemorygenerator.InMemoryGenerator).Algorithm).To(Equal("rsa"))
			})

			It("succeds if the default generator certs expires in 365 days", func() {
				Expect(defaultGenerator.(*inmemorygenerator.InMemoryGenerator).Expiry).To(Equal(365))
			})
		})
	})
})
