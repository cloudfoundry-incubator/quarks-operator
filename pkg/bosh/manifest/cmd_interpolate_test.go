package manifest_test

import (
	"io/ioutil"
	"path/filepath"

	"go.uber.org/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("InterpolateVariables", func() {

	var (
		baseManifest   []byte
		varDir         string
		log            *zap.SugaredLogger
		outputFilePath string
	)
	BeforeEach(func() {
		_, log = helper.NewTestLogger()
		baseManifest = []byte(`
---
director_uuid: ((password1))
instance_groups:
- name: ((value1.key1))
- name: ((value2.key2))
- name: ((value2.key3))
`)
		varDir = filepath.Join(assetPath, "vars")
		outputFilePath = filepath.Join(assetPath, "output.json")

	})

	It("returns interpolated manifest", func() {
		err := InterpolateVariables(log, baseManifest, varDir, outputFilePath)
		Expect(err).NotTo(HaveOccurred())

		dataBytes, err := ioutil.ReadFile(outputFilePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(err).To(BeNil())

		Expect(string(dataBytes)).To(Equal(`{"manifest.yaml":"director_uuid: |\n  fake-password\ninstance_groups:\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings:\n          ephemeralAsPVC: false\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    baz\n  properties:\n    quarks: {}\n  stemcell: \"\"\n  vm_resources: null\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings:\n          ephemeralAsPVC: false\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    foo\n  properties:\n    quarks: {}\n  stemcell: \"\"\n  vm_resources: null\n- azs: null\n  env:\n    bosh:\n      agent:\n        settings:\n          ephemeralAsPVC: false\n      ipv6:\n        enable: false\n  instances: 0\n  jobs: null\n  name: |\n    bar\n  properties:\n    quarks: {}\n  stemcell: \"\"\n  vm_resources: null\n"}`))
	})

	It("raises error when variablesDir is not directory", func() {
		varDir = assetPath + "/nonexisting"
		err := InterpolateVariables(log, baseManifest, varDir, outputFilePath)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not read variables directory"))
	})
})
