package bpm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	bpm "code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

var _ = Describe("BPM Config", func() {
	Describe("NewConfig", func() {
		var (
			yaml []byte
		)

		BeforeEach(func() {
			yaml = []byte(`processes:
  - name: server
    executable: /var/vcap/data/packages/server/serve.sh
    args:
    - --port
    - 2424
    env:
      FOO: BAR
    limits:
      processes: 10
    ephemeral_disk: true
    additional_volumes:
    - path: /var/vcap/data/sockets
      writable: true
    capabilities:
    - NET_BIND_SERVICE
  - name: worker
    executable: /var/vcap/data/packages/worker/work.sh
    args:
    - --queues
    - 4
    additional_volumes:
    - path: /var/vcap/data/sockets
      writable: true
    hooks:
      pre_start: /var/vcap/jobs/server/bin/worker-setup`)
		})

		It("unmarshals all data", func() {
			config, err := bpm.NewConfig(yaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(config.Processes)).To(Equal(2))

			By("Unmarshalling the server process")
			serverProcess := config.Processes[0]
			Expect(serverProcess.Executable).To(Equal("/var/vcap/data/packages/server/serve.sh"))
			Expect(serverProcess.Args).To(Equal([]string{"--port", "2424"}))
			Expect(serverProcess.Env).To(Equal(map[string]string{"FOO": "BAR"}))
			Expect(serverProcess.Limits).To(Equal(bpm.Limits{Processes: 10, Memory: "", OpenFiles: 0}))
			Expect(serverProcess.EphemeralDisk).To(BeTrue())
			Expect(serverProcess.AdditionalVolumes).To(Equal([]bpm.Volume{bpm.Volume{Path: "/var/vcap/data/sockets", Writable: true}}))
			Expect(serverProcess.Capabilities).To(Equal([]string{"NET_BIND_SERVICE"}))

			By("Unmarshalling the worker process")
			workerProcess := config.Processes[1]
			Expect(workerProcess.Executable).To(Equal("/var/vcap/data/packages/worker/work.sh"))
			Expect(workerProcess.Args).To(Equal([]string{"--queues", "4"}))
			Expect(workerProcess.AdditionalVolumes).To(Equal([]bpm.Volume{bpm.Volume{Path: "/var/vcap/data/sockets", Writable: true}}))
			Expect(workerProcess.Hooks).To(Equal(bpm.Hooks{PreStart: "/var/vcap/jobs/server/bin/worker-setup"}))
		})
	})

	Describe("MergeEnv", func() {
		var (
			process   bpm.Process
			overrides []corev1.EnvVar
		)

		BeforeEach(func() {
			process = bpm.Process{}
			overrides = []corev1.EnvVar{}
		})

		Context("when process env is empty", func() {
			Context("when override is empty", func() {
				It("returns nil", func() {
					vars := process.MergeEnv(overrides)
					Expect(vars).To(BeNil())
					Expect(vars).To(HaveLen(0))
				})
			})

			Context("when override is present", func() {
				BeforeEach(func() {
					overrides = []corev1.EnvVar{
						corev1.EnvVar{Name: "first", Value: "data"},
						corev1.EnvVar{Name: "second", Value: "", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "foo"}}},
					}

				})

				It("returns the override EnvVars", func() {
					vars := process.MergeEnv(overrides)
					Expect(vars).To(HaveLen(2))
				})
			})
		})

		Context("when process env is present", func() {
			BeforeEach(func() {
				process = bpm.Process{Env: map[string]string{
					"org": "org-data",
				}}
			})

			Context("when override is empty", func() {
				It("returns process env", func() {
					vars := process.MergeEnv(overrides)
					Expect(vars).To(HaveLen(1))
				})
			})

			Context("when override is present", func() {
				BeforeEach(func() {
					overrides = []corev1.EnvVar{
						corev1.EnvVar{Name: "first", Value: "new-data"},
						corev1.EnvVar{Name: "org", Value: "over-data"},
					}

				})

				It("returns a union and with override taking precedence", func() {
					vars := process.MergeEnv(overrides)
					Expect(vars).To(HaveLen(2))
					data := []string{}
					for _, env := range vars {
						data = append(data, env.Value)
					}
					Expect(data).To(ContainElement("new-data"))
					Expect(data).To(ContainElement("over-data"))
				})
			})
		})
	})
})
