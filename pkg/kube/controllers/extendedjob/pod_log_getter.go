package extendedjob

import (
	corev1 "k8s.io/api/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// PodLogGetter fetches the logs from a Pod
type PodLogGetter interface {
	Get(namespace, podName, containerName string) ([]byte, error)
}

// NewPodLogGetter returns an instance of a PodLogGetterImpl
func NewPodLogGetter(client corev1client.CoreV1Interface) PodLogGetter {
	return &PodLogGetterImpl{client: client}
}

// PodLogGetterImpl implements the PodLogGetter interface
type PodLogGetterImpl struct {
	client corev1client.CoreV1Interface
}

// Get fetches the logs for the given pod
func (p *PodLogGetterImpl) Get(namespace, podName, containerName string) ([]byte, error) {
	options := corev1.PodLogOptions{
		Container: containerName,
	}
	logs, err := p.client.Pods(namespace).GetLogs(podName, &options).DoRaw()
	if err != nil {
		return []byte{}, err
	}

	return logs, nil
}
