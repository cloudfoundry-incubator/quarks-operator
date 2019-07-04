package manifest_test

import (
	"io/ioutil"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//. "github.com/onsi/gomega/gstruct"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

var _ = Describe("BOSHContainerization", func() {
	var (
		m *Manifest
	)

	BeforeEach(func() {
		manifest_path := path.Join(assetPath, "gatherManifest.yml")

		boshManifestBytes, err := ioutil.ReadFile(manifest_path)
		Expect(err).ToNot(HaveOccurred())

		m, err = LoadYAML(boshManifestBytes)
		Expect(err).ToNot(HaveOccurred())
	})

	It("parses the readiness probe in the run configuration", func() {
		ig, err := m.InstanceGroupByName("doppler")
		Expect(err).ToNot(HaveOccurred())

		healthchecks := ig.Jobs[0].Properties.BOSHContainerization.Run.HealthChecks
		Expect(len(healthchecks)).To(Equal(1))
		Expect(healthchecks["doppler"].ReadinessProbe.Exec.Command[0]).To(Equal("curl --silent --fail --head http://${HOSTNAME}:8080/health"))
		Expect(healthchecks["doppler"].LivenessProbe).To(BeNil())
	})

	It("parses the liveness probe in the run configuration", func() {
		ig, err := m.InstanceGroupByName("log-api")
		Expect(err).ToNot(HaveOccurred())

		healthchecks := ig.Jobs[0].Properties.BOSHContainerization.Run.HealthChecks
		Expect(len(healthchecks)).To(Equal(1))
		Expect(healthchecks["doppler"].LivenessProbe.Exec.Command[0]).To(Equal("curl --silent --fail --head http://${HOSTNAME}:8080/health"))
		Expect(healthchecks["doppler"].ReadinessProbe).To(BeNil())
	})
})
