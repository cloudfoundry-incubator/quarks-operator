package environment

// The functions in this file were only used by the extended statefulset
// components tests, but might be used by others now.  They were split off in
// preparation for standalone components.

import (
	"context"
	"encoding/json"
	"os"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// GetStatefulSet gets a StatefulSet by namespace and name
func (m *Machine) GetStatefulSet(namespace string, name string) (*appsv1.StatefulSet, error) {
	statefulSet, err := m.Clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return &appsv1.StatefulSet{}, errors.Wrapf(err, "failed to query for statefulSet by name: %v", name)
	}

	return statefulSet, nil
}

// WaitForPodLabelToExist blocks until the specified label appears
func (m *Machine) WaitForPodLabelToExist(n string, podName string, label string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.PodLabelToExist(n, podName, label)
	})
}

// PodLabelToExist returns true if the label exist in the specified pod
func (m *Machine) PodLabelToExist(n string, podName string, label string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(n).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	labels := pod.GetLabels()
	if _, found := labels[label]; found {
		return true, nil
	}
	return false, nil
}

// WaitForPodLabelToNotExist blocks until the specified label is not present
func (m *Machine) WaitForPodLabelToNotExist(n string, podName string, label string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.PodLabelToNotExist(n, podName, label)
	})
}

// PodLabelToNotExist returns true if the label does not exist
func (m *Machine) PodLabelToNotExist(n string, podName string, label string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(n).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	labels := pod.GetLabels()
	if _, found := labels[label]; !found {
		return true, nil
	}
	return false, nil
}

// WaitForStatefulSetDelete blocks until the specified statefulSet is deleted
func (m *Machine) WaitForStatefulSetDelete(namespace string, name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		found, err := m.StatefulSetExist(namespace, name)
		return !found, err
	})
}

// StatefulSetExist checks if the statefulSet exists
func (m *Machine) StatefulSetExist(namespace string, name string) (bool, error) {
	_, err := m.Clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for statefulSet by name: %s", name)
	}
	return true, nil
}

// WaitForStatefulSetNewGeneration blocks until at least one StatefulSet is
// found, which has a generation greater than currentVersion. It fails after
// the timeout.
func (m *Machine) WaitForStatefulSetNewGeneration(namespace string, name string, currentVersion int64) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.StatefulSetNewGeneration(namespace, name, currentVersion)
	})
}

// WaitForPV blocks until the pv is running. It fails after the timeout.
func (m *Machine) WaitForPV(name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.PVAvailable(name)
	})
}

// PVAvailable returns true if the pv by that name is in state available
func (m *Machine) PVAvailable(name string) (bool, error) {
	pv, err := m.Clientset.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pv by name: %s", name)
	}

	if pv.Status.Phase == "Available" {
		return true, nil
	}
	return false, nil
}

// WaitForPVsDelete blocks until the pv is deleted. It fails after the timeout.
func (m *Machine) WaitForPVsDelete(labels string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.PVsDeleted(labels)
	})
}

// PVsDeleted returns true if the all pvs are deleted
func (m *Machine) PVsDeleted(labels string) (bool, error) {
	pvList, err := m.Clientset.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return false, err
	}
	if len(pvList.Items) == 0 {
		return true, nil
	}
	return false, nil
}

// WaitForPVCsDelete blocks until the pvc is deleted. It fails after the timeout.
func (m *Machine) WaitForPVCsDelete(namespace string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.PVCsDeleted(namespace)
	})
}

// PVCsDeleted returns true if the all pvs are deleted
func (m *Machine) PVCsDeleted(namespace string) (bool, error) {
	pvcList, err := m.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	if len(pvcList.Items) == 0 {
		return true, nil
	}
	return false, nil
}

// WaitForStatefulSet blocks until all statefulSet pods are running. It fails after the timeout.
func (m *Machine) WaitForStatefulSet(namespace string, name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.StatefulSetRunning(namespace, name)
	})
}

// StatefulSetRunning returns true if the statefulSet by that name has all pods created
func (m *Machine) StatefulSetRunning(namespace string, name string) (bool, error) {
	statefulSet, err := m.Clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for statefulSet by name: %s", name)
	}

	if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
		return true, nil
	}
	return false, nil
}

// WaitForQuarksStatefulSetDelete blocks until the specified quarksstatefulSet is deleted
func (m *Machine) WaitForQuarksStatefulSetDelete(namespace string, name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		found, err := m.QuarksStatefulSetExists(namespace, name)
		return !found, err
	})
}

// QuarksStatefulSetExists returns true if at least one ess selected by labels exists
func (m *Machine) QuarksStatefulSetExists(namespace string, name string) (bool, error) {
	_, err := m.QuarksStatefulSetClient.QuarksstatefulsetV1alpha1().QuarksStatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for quarksstatefulSet by name: %s", name)
	}

	return true, nil
}

// StatefulSetNewGeneration returns true if StatefulSet has a new generation greater `generation`
func (m *Machine) StatefulSetNewGeneration(namespace string, name string, generation int64) (bool, error) {
	client := m.Clientset.AppsV1().StatefulSets(namespace)

	ss, err := client.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for statefulSet by name: %v", name)
	}

	if ss.Status.ObservedGeneration > generation {
		return true, nil
	}

	return false, nil
}

// GetNamespaceEvents exits as soon as an event reason and msg matches
func (m *Machine) GetNamespaceEvents(namespace, name, id, reason, msg string) (bool, error) {
	eList, err := m.Clientset.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fields.Set{
			"involvedObject.name": name,
			"involvedObject.uid":  id,
		}.AsSelector().String(),
	})
	if err != nil {
		return false, err
	}
	// Loop here due to the size
	// of the events in the ns
	for _, n := range eList.Items {
		if n.Reason == reason && n.Message == msg {
			return true, nil
		}
	}

	return false, nil
}

// ExecPodCMD executes a cmd in a container
func (m *Machine) ExecPodCMD(client kubernetes.Interface, rc *rest.Config, pod *corev1.Pod, container string, command []string) (bool, error) {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, clientscheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(rc, "POST", req.URL())
	if err != nil {
		return false, errors.New("failed to initialize remote command executor")
	}
	if err = executor.Stream(remotecommand.StreamOptions{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Tty: false}); err != nil {
		return false, errors.Wrapf(err, "failed executing command in pod: %s, container: %s in namespace: %s",
			pod.Name,
			container,
			pod.Namespace,
		)
	}
	return true, nil
}

// PatchPod applies a patch into a specific pod
// operation can be of the form add,remove,replace
// See https://tools.ietf.org/html/rfc6902 for more information
func (m *Machine) PatchPod(namespace string, name string, o string, p string, v string) error {

	payloadBytes, _ := json.Marshal(
		[]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value string `json:"value"`
		}{{
			Op:    o,
			Path:  p,
			Value: v,
		}},
	)

	_, err := m.Clientset.CoreV1().Pods(namespace).Patch(
		context.Background(),
		name,
		types.JSONPatchType,
		payloadBytes,
		metav1.PatchOptions{},
	)

	return err
}
