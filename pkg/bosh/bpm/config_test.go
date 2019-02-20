package bpm_test

import (
	"fmt"

	bpm "code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bpm Config", func() {
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
		fmt.Println(string(yaml))
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
