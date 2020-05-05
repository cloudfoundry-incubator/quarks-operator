package manifest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	. "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("Addons", func() {
	var (
		manifest *Manifest
		logs     *observer.ObservedLogs
		log      *zap.SugaredLogger
	)

	BeforeEach(func() {
		logger.Trace = false
	})

	JustBeforeEach(func() {
		var err error
		manifest, err = LoadYAML([]byte(boshmanifest.WithAddons))
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest).ToNot(BeNil())

		logs, log = helper.NewTestLogger()
		log = logger.TraceFilter(log, "test")
	})

	It("should add addon jobs to instance groups", func() {
		err := manifest.ApplyAddons(log)
		Expect(err).NotTo(HaveOccurred())
		Expect(logs.All()).To(HaveLen(0))

		Expect(manifest.InstanceGroups).To(HaveLen(3))
		Expect(manifest.InstanceGroups[0].Jobs).To(HaveLen(3))
		Expect(manifest.InstanceGroups[0].Jobs[0].Name).To(Equal("redis-server"))
		Expect(manifest.InstanceGroups[0].Jobs[1].Name).To(Equal("addon-job"))
		Expect(manifest.InstanceGroups[0].Jobs[2].Name).To(Equal("addon-job2"))
		Expect(manifest.InstanceGroups[1].Jobs).To(HaveLen(2))
		Expect(manifest.InstanceGroups[1].Jobs[0].Name).To(Equal("cflinuxfs3-rootfs-setup"))
		Expect(manifest.InstanceGroups[1].Jobs[1].Name).To(Equal("addon-job"))
		Expect(manifest.InstanceGroups[2].Jobs).To(HaveLen(2))
		Expect(manifest.InstanceGroups[2].Jobs[0].Name).To(Equal("redis-server"))
		Expect(manifest.InstanceGroups[2].Jobs[1].Name).To(Equal("addon-job3"))
	})

	Context("when using trace logger", func() {
		BeforeEach(func() {
			logger.Trace = true
		})

		It("should log", func() {
			err := manifest.ApplyAddons(log)
			Expect(err).NotTo(HaveOccurred())

			Expect(logs.FilterMessageSnippet("'redis-slave-errand' is an errand, but the exclusion placement rules don't match").Len()).To(Equal(3))
		})
	})
})
