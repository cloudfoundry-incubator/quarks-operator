package manifest_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/testing/boshmanifest"
)

var _ = Describe("Addons", func() {
	It("should add addon jobs to instance groups", func() {
		manifest, err := LoadYAML([]byte(boshmanifest.WithAddons))
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest).ToNot(BeNil())

		err = manifest.ApplyAddons(context.Background())
		Expect(err).NotTo(HaveOccurred())

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
})
