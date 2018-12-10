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

	Describe("GenerateSSHKey", func() {
		It("generates an SSH key", func() {
			key, err := generator.GenerateSSHKey("foo")

			Expect(err).ToNot(HaveOccurred())
			Expect(key.PrivateKey).To(ContainSubstring("BEGIN RSA PRIVATE KEY"))
			Expect(key.PublicKey).To(MatchRegexp("ssh-rsa\\s.+"))
			Expect(key.Fingerprint).To(MatchRegexp("([0-9a-f]{2}:){15}[0-9a-f]{2}"))
		})
	})
})
