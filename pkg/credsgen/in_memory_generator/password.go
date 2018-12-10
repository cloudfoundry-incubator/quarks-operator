package inmemorygenerator

import (
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"github.com/dchest/uniuri"
)

func (g InMemoryGenerator) GeneratePassword(name string, request credsgen.PasswordGenerationRequest) string {
	fmt.Println("Generating password ", name)

	length := request.Length
	if length == 0 {
		length = credsgen.DefaultPasswordLength
	}

	return uniuri.NewLen(length)
}
