package environment

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	pollInterval = 1 * time.Second
)

// GetPodLogs gets pod logs
func (m *Machine) GetPodLogs(namespace, podName string) (string, error) {
	return m.podLogs(namespace, podName, corev1.PodLogOptions{})
}

// GetPodContainerLogs gets logs for a specific container in a pd
func (m *Machine) GetPodContainerLogs(namespace, podName, containerName string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}

	return m.podLogs(namespace, podName, podLogOpts)
}

func (m *Machine) podLogs(namespace, podName string, opts corev1.PodLogOptions) (string, error) {
	req := m.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &opts)
	podLogs, err := req.Stream()
	if err != nil {
		return "", errors.Wrapf(err, "error opening log stream for pod")
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", errors.Wrapf(err, "failed to copy bytes from the pod log to a buffer")
	}
	str := buf.String()

	return str, nil
}

// WaitForPodLogMsg searches pod test logs for at least one occurrence of msg.
func (m *Machine) WaitForPodLogMsg(namespace string, podName string, msg string) error {
	return wait.PollImmediate(pollInterval, m.pollTimeout, func() (bool, error) {
		logs, err := m.GetPodLogs(namespace, podName)
		return strings.Contains(logs, msg), err
	})
}

// WaitForPodContainerLogMsg searches pod test logs for at least one occurrence of msg.
func (m *Machine) WaitForPodContainerLogMsg(namespace, podName, containerName, msg string) error {
	return wait.PollImmediate(100*time.Millisecond, m.pollTimeout, func() (bool, error) {
		logs, err := m.GetPodContainerLogs(namespace, podName, containerName)
		return strings.Contains(logs, msg), err
	})
}

// PodContainsLogMsg searches pod test logs for at least one occurrence of msg, but it will
// have a shorter timeout(10secs). This is for tests where one does not expect to see a log.
func (m *Machine) PodContainsLogMsg(namespace, podName, containerName, msg string) error {
	return wait.PollImmediate(pollInterval, 10*time.Second, func() (bool, error) {
		logs, err := m.GetPodContainerLogs(namespace, podName, containerName)
		return strings.Contains(logs, msg), err
	})
}

// WaitForPodLogMatchRegexp searches pod test logs for at least one occurrence of Regexp.
func (m *Machine) WaitForPodLogMatchRegexp(namespace string, podName string, regExp string) error {
	r, _ := regexp.Compile(regExp)

	return wait.PollImmediate(pollInterval, m.pollTimeout, func() (bool, error) {
		logs, err := m.GetPodLogs(namespace, podName)
		return r.MatchString(logs), err
	})
}

// WaitForPodContainerLogMatchRegexp searches a pod's container test logs for at least one occurrence of Regexp.
func (m *Machine) WaitForPodContainerLogMatchRegexp(namespace string, podName string, containerName string, regExp string) error {
	r, _ := regexp.Compile(regExp)

	return wait.PollImmediate(pollInterval, m.pollTimeout, func() (bool, error) {
		logs, err := m.GetPodContainerLogs(namespace, podName, containerName)
		return r.MatchString(logs), err
	})
}

// WaitForLogMsg searches zap test logs for at least one occurrence of msg.
// When using this, tests should use FlushLog() to remove log messages from
// other tests.
func (m *Machine) WaitForLogMsg(logs *observer.ObservedLogs, msg string) error {
	return wait.PollImmediate(pollInterval, m.pollTimeout, func() (bool, error) {
		n := logs.FilterMessageSnippet(msg).Len()
		return n > 0, nil
	})
}
