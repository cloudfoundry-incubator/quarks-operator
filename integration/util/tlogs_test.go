package util_test

import (
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/operatorimage"
	"code.cloudfoundry.org/quarks-utils/testing/machine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("when testing tail-logs subcommand", func() {

	Context("subcommand must be working", func() {
		podName := "test-pod-bar-foo"
		parentCName := "fake-nats"
		sidecarCName := "logs"

		It("when tailing and only one file exists", func() {

			scriptCreateDirs := fmt.Sprint(`mkdir -p /var/vcap/sys/log/nats;
			touch /var/vcap/sys/log/nats/nats.log;
			while true;
			do echo "nats-msg-line" >> /var/vcap/sys/log/nats/nats.log; sleep 5;
			done`)

			testPod := env.PodWithTailLogsContainer(podName, scriptCreateDirs, parentCName, sidecarCName, operatorimage.GetOperatorDockerImage())

			tearDown, err := env.CreatePod(env.Namespace, testPod)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			err = env.WaitForPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodContainerLogMsg(env.Namespace, podName, sidecarCName, "nats-msg-line")
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodContainerLogMsg(env.Namespace, podName, sidecarCName, "/var/vcap/sys/log/nats/nats.log")
			Expect(err).NotTo(HaveOccurred())
		})

		It("when tailing and more than one file exists", func() {
			scriptCreateDirs := fmt.Sprint(`mkdir -p /var/vcap/sys/log/nats;
			mkdir -p /var/vcap/sys/log/doppler;
			touch /var/vcap/sys/log/nats/nats.log;
			touch /var/vcap/sys/log/doppler/doppler.log
			while true;
			do echo "nats-msg-line" >> /var/vcap/sys/log/nats/nats.log; sleep 5;
			echo "doppler-msg-line" >> /var/vcap/sys/log/doppler/doppler.log; sleep 5;
			done`)

			testPod := env.PodWithTailLogsContainer(podName, scriptCreateDirs, parentCName, sidecarCName, operatorimage.GetOperatorDockerImage())

			tearDown, err := env.CreatePod(env.Namespace, testPod)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			err = env.WaitForPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodContainerLogMsg(env.Namespace, podName, sidecarCName, "nats-msg-line")
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForPodContainerLogMsg(env.Namespace, podName, sidecarCName, "doppler-msg-line")
			Expect(err).NotTo(HaveOccurred())

		})
		It("when tailing and an unsupported files exist", func() {
			scriptCreateDirs := fmt.Sprint(`mkdir -p /var/vcap/sys/log/nats;
			touch /var/vcap/sys/log/nats/nats.log;
			touch /var/vcap/sys/log/nats/nats.err
			while true;
			do echo "nats-msg-line" >> /var/vcap/sys/log/nats/nats.log; sleep 5;
			echo "nats-error-msg-line" >> /var/vcap/sys/log/nats/nats.err; sleep 5;
			done`)

			testPod := env.PodWithTailLogsContainer(podName, scriptCreateDirs, parentCName, sidecarCName, operatorimage.GetOperatorDockerImage())

			tearDown, err := env.CreatePod(env.Namespace, testPod)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			err = env.WaitForPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())

			err = env.PodContainsLogMsg(env.Namespace, podName, sidecarCName, "nats-error-msg-line")
			Expect(err).To(HaveOccurred())

		})
		It("when tailing and no file exist", func() {
			scriptCreateDirs := fmt.Sprint(`
			mkdir -p /var/vcap/sys/log
			while true;
			do sleep 5;
			done`)

			testPod := env.PodWithTailLogsContainer(podName, scriptCreateDirs, parentCName, sidecarCName, operatorimage.GetOperatorDockerImage())

			tearDown, err := env.CreatePod(env.Namespace, testPod)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			err = env.WaitForPod(env.Namespace, podName)
			Expect(err).NotTo(HaveOccurred())

		})
	})
})
