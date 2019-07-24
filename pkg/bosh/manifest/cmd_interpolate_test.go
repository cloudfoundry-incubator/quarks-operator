package manifest_test

import (
	"go.uber.org/zap"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("InterpolateVariables", func() {

	var (
		baseManifest []byte
		varDir       string
		log          *zap.SugaredLogger
	)
	BeforeEach(func() {
		_, log = helper.NewTestLogger()
		baseManifest = []byte(`
---
password: ((password1))
instance-group:
  key1: ((value1.key1))
  key2: ((value2.key2))
  key3: ((value2.key3))
`)
		varDir = assetPath + "/vars"

	})

	invoke := func(expectedErr string) (string, error) {
		r, w, _ := os.Pipe()
		tmp := os.Stdout
		defer func() {
			os.Stdout = tmp
		}()
		os.Stdout = w

		go func() {
			err := InterpolateVariables(log, baseManifest, varDir)
			if len(expectedErr) != 0 {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(expectedErr))
			} else {
				Expect(err).To(BeNil())
			}
			w.Close()
		}()
		stdout, err := ioutil.ReadAll(r)
		return string(stdout), err
	}

	It("returns interpolated manifest", func() {
		stdOut, err := invoke("")
		Expect(err).To(BeNil())
		Expect(stdOut).To(Equal(`{"manifest.yaml":"instance-group:\n  key1: |\n    baz\n  key2: |\n    foo\n  key3: |\n    bar\npassword: |\n  fake-password\n"}`))
	})

	It("raises error when variablesDir is not directory", func() {
		varDir = assetPath + "/nonexisting"
		_, err := invoke("could not read variables directory")
		Expect(err).To(BeNil())
	})
})
