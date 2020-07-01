package kube_test

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/yaml"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("K8s native resources provide BOSH links to a BOSH deployment", func() {
	kubectl = cmdHelper.NewKubectl()

	jobLink := func(name string) manifest.JobLink {
		enc, err := cmdHelper.GetData(namespace, "secret", "ig-resolved.quarks-gora-v1", `go-template={{index .data "properties.yaml"}}`)
		Expect(err).ToNot(HaveOccurred())
		decoded, _ := base64.StdEncoding.DecodeString(string(enc))

		manifest := &manifest.Manifest{}
		err = yaml.Unmarshal([]byte(decoded), manifest)
		Expect(err).ToNot(HaveOccurred())
		Expect(manifest.InstanceGroups).To(HaveLen(1))
		Expect(manifest.InstanceGroups[0].Jobs).To(HaveLen(1))
		return manifest.InstanceGroups[0].Jobs[0].Properties.Quarks.Consumes[name]
	}

	Context("when the link has an underscore in its name", func() {
		BeforeEach(func() {
			apply("quarks-link/native-to-bosh/underscore.yaml")
			err := kubectl.WaitForSecret(namespace, "ig-resolved.quarks-gora-v1")
			Expect(err).ToNot(HaveOccurred())
		})

		It("uses kube native link", func() {
			link := jobLink("quarks_gora")
			fmt.Printf("%#v\n", link)
			Expect(link.Address).To(ContainSubstring("svcexternal." + namespace + ".svc."))

			By("Checking instance", func() {
				Expect(link.Instances).To(HaveLen(1))
				instance := link.Instances[0]
				Expect(instance.Address).To(ContainSubstring("svcexternal." + namespace + ".svc."))
				Expect(instance.AZ).To(BeEmpty())
				Expect(instance.Bootstrap).To(Equal(true))
				Expect(instance.ID).To(Equal(`quarks_gora`))
				Expect(instance.Instance).To(BeZero())
				Expect(instance.Index).To(BeZero())
				Expect(instance.Name).To(Equal(`quarks_gora`))
			})

			By("Checking properties", func() {
				props := link.Properties
				Expect(props).To(HaveKeyWithValue("text_message", "admin"))
				Expect(props).To(HaveKey("quarks-gora"))
				gora := props["quarks-gora"]
				Expect(gora).To(HaveKeyWithValue("port", "1234"))
				Expect(gora).To(HaveKeyWithValue("ssl", false))
			})
		})
	})

	Context("when gora smoke test uses a native provider", func() {
		BeforeEach(func() {
			apply("quarks-link/native-to-bosh/link-secret.yaml")
		})

		JustBeforeEach(func() {
			// after creating the service, create a deployment to assert against
			apply("quarks-link/native-to-bosh/boshdeployment.yaml")
			err := kubectl.WaitForSecret(namespace, "ig-resolved.quarks-gora-v1")
			Expect(err).ToNot(HaveOccurred())

		})

		Context("when the service has a selector", func() {
			BeforeEach(func() {
				apply("quarks-link/native-to-bosh/link-pod.yaml")
				apply("quarks-link/native-to-bosh/link-service.yaml")
			})

			It("uses kube native link", func() {
				ip, err := cmdHelper.GetData(namespace, "pod", "linkpod-0", "go-template={{.status.podIP}}")
				Expect(err).ToNot(HaveOccurred())

				link := jobLink("quarks-gora")
				Expect(link.Address).To(ContainSubstring("testservice." + namespace + ".svc."))

				By("Checking instance", func() {
					Expect(link.Instances).To(HaveLen(1))
					instance := link.Instances[0]
					Expect(instance.Address).To(ContainSubstring(string(ip)))
					Expect(instance.AZ).To(BeEmpty())
					Expect(instance.Bootstrap).To(Equal(true))
					Expect(instance.ID).ToNot(BeEmpty())
					Expect(instance.Instance).To(BeZero())
					Expect(instance.Index).To(BeZero())
					Expect(instance.Name).To(Equal(`quarks-gora`))
				})

				By("Checking properties", func() {
					props := link.Properties
					Expect(props).To(HaveKeyWithValue("text_message", "admin"))
					Expect(props).To(HaveKey("quarks-gora"))
					gora := props["quarks-gora"]
					Expect(gora).To(HaveKeyWithValue("port", "1234"))
					Expect(gora).To(HaveKeyWithValue("ssl", false))
				})
			})
		})

		Context("when the service points to an external name", func() {
			BeforeEach(func() {
				apply("quarks-link/native-to-bosh/link-service-external-name.yaml")
			})

			It("uses kube native link", func() {
				link := jobLink("quarks-gora")
				Expect(link.Address).To(ContainSubstring("svcexternal." + namespace + ".svc."))

				By("Checking instance", func() {
					Expect(link.Instances).To(HaveLen(1))
					instance := link.Instances[0]
					Expect(instance.Address).To(ContainSubstring("svcexternal." + namespace + ".svc."))
					Expect(instance.AZ).To(BeEmpty())
					Expect(instance.Bootstrap).To(Equal(true))
					Expect(instance.ID).To(Equal(`quarks-gora`))
					Expect(instance.Instance).To(BeZero())
					Expect(instance.Index).To(BeZero())
					Expect(instance.Name).To(Equal(`quarks-gora`))
				})

				By("Checking properties", func() {
					props := link.Properties
					Expect(props).To(HaveKeyWithValue("text_message", "admin"))
					Expect(props).To(HaveKey("quarks-gora"))
					gora := props["quarks-gora"]
					Expect(gora).To(HaveKeyWithValue("port", "1234"))
					Expect(gora).To(HaveKeyWithValue("ssl", false))
				})
			})
		})

		Context("when the service points to an endpoint", func() {
			BeforeEach(func() {
				apply("quarks-link/native-to-bosh/link-service-endpoint.yaml")
			})

			It("uses kube native link", func() {
				link := jobLink("quarks-gora")
				Expect(link.Address).To(ContainSubstring("endpointsvc." + namespace + ".svc."))

				By("Checking instance", func() {
					Expect(link.Instances).To(HaveLen(1))
					instance := link.Instances[0]
					Expect(instance.Address).To(Equal("192.1.2.34"))
					Expect(instance.AZ).To(BeEmpty())
					Expect(instance.Bootstrap).To(Equal(true))
					Expect(instance.ID).To(Equal("192.1.2.34"))
					Expect(instance.Instance).To(BeZero())
					Expect(instance.Index).To(BeZero())
					Expect(instance.Name).To(Equal(`quarks-gora`))
				})

				By("Checking properties", func() {
					props := link.Properties
					Expect(props).To(HaveKeyWithValue("text_message", "admin"))
					Expect(props).To(HaveKey("quarks-gora"))
					gora := props["quarks-gora"]
					Expect(gora).To(HaveKeyWithValue("port", "1234"))
					Expect(gora).To(HaveKeyWithValue("ssl", false))
				})
			})
		})
	})
})
