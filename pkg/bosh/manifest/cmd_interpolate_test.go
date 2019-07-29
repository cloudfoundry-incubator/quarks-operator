package manifest_test

import (
	"io/ioutil"
	"os"

	"go.uber.org/zap"

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
name: ((password1))
instance_groups:
- name: ((value1.key1))
- name: ((value2.key2))
- name: ((value2.key3))
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

		Expect(stdOut).To(Equal(`{"manifest.yaml":"director_uuid: \"\"\ninstance_groups:\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings: {}\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    baz\n  stemcell: \"\"\n  vm_resources: null\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings: {}\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    foo\n  stemcell: \"\"\n  vm_resources: null\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings: {}\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    bar\n  stemcell: \"\"\n  vm_resources: null\nname: |\n  fake-password\n"}`))
	})

	It("raises error when variablesDir is not directory", func() {
		varDir = assetPath + "/nonexisting"
		_, err := invoke("could not read variables directory")
		Expect(err).To(BeNil())
	})
})
