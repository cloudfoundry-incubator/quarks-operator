package testing

import (
	"bytes"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Kubectl is used as a command to test e2e tests
type Kubectl struct {
	Log          *zap.SugaredLogger
	Namespace    string
	pollTimeout  time.Duration
	pollInterval time.Duration
}

// NewKubectl returns a new CfOperatorkubectl command
func NewKubectl() *Kubectl {
	return &Kubectl{
		Namespace:    "",
		pollTimeout:  300 * time.Second,
		pollInterval: 500 * time.Millisecond,
	}
}

// CreateNamespace create the namespace using kubectl command
func (k *Kubectl) CreateNamespace(name string) error {
	cmd := exec.Command("kubectl", "create", "namespace", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// DeleteNamespace deletes the namespace using kubectl command
func (k *Kubectl) DeleteNamespace(name string) error {
	cmd := exec.Command("kubectl", "delete", "namespace", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// Create creates the resource using kubectl command
func (k *Kubectl) Create(namespace string, yamlFilePath string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "create", "-f", yamlFilePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// Apply updates the resource using kubectl command
func (k *Kubectl) Apply(namespace string, yamlFilePath string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "apply", "-f", yamlFilePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// RunCommandWithCheckString runs the command specified helperin the container
func (k *Kubectl) RunCommandWithCheckString(namespace string, podName string, commandInPod string, result string) error {
	return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
		return k.CheckString(namespace, podName, commandInPod, result)
	})
}

// CheckString checks is the string is present in the output of the kubectl command
func (k *Kubectl) CheckString(namespace string, podName string, commandInPod string, result string) (bool, error) {
	out, err := exec.Command("kubectl", "--namespace", namespace, "exec", "-it", podName, commandInPod).Output()
	if err != nil {
		return false, nil
	}
	if strings.Contains(string(out), result) {
		return true, nil
	}
	return false, nil
}

// RunCommandWithOutput runs the command specified in the container and returns outpu
func (k *Kubectl) RunCommandWithOutput(namespace string, podName string, commandInPod string) (string, error) {
	kubectlCommand := "kubectl --namespace " + namespace + " exec -it " + podName + " " + commandInPod
	cmd := exec.Command("bash", "-c", kubectlCommand)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", errors.Wrapf(err, stderr.String())
	}
	if len(out.String()) > 0 {
		return out.String(), nil
	}
	return "", err
}

// GetSecretData fetches the specified output by the given templatePath
func (k *Kubectl) GetSecretData(namespace string, secretName string, templatePath string) ([]byte, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "secret", secretName, "-o", templatePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return []byte{}, errors.Wrapf(err, string(out))
	}
	if len(string(out)) > 0 {
		return out, nil
	}
	return []byte{}, errors.Wrapf(err, "Output is empty")
}

// WaitForSecret blocks until the secret is available. It fails after the timeout.
func (k *Kubectl) WaitForSecret(namespace string, secretName string) error {
	return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
		return k.SecretExists(namespace, secretName)
	})
}

// SecretExists returns true if the pod by that name is in state running
func (k *Kubectl) SecretExists(namespace string, secretName string) (bool, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "secret", secretName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, errors.Wrapf(err, string(out))
	}
	if strings.Contains(string(out), secretName) {
		return true, nil
	}
	return false, nil
}

// WaitForPVC blocks until the pvc is available. It fails after the timeout.
func (k *Kubectl) WaitForPVC(namespace string, pvcName string) error {
	return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
		return k.PVCExists(namespace, pvcName)
	})
}

// PVCExists returns true if the pvc by that name exists
func (k *Kubectl) PVCExists(namespace string, pvcName string) (bool, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "pvc", pvcName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, errors.Wrapf(err, string(out))
	}
	if strings.Contains(string(out), pvcName) {
		return true, nil
	}
	return false, nil
}

// Wait waits for the condition on the resource using kubectl command
func (k *Kubectl) Wait(namespace string, requiredStatus string, resourceName string) error {
	err := wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
		return k.CheckWait(namespace, requiredStatus, resourceName)
	})

	if err != nil {
		return errors.Wrapf(err, "current stack: %s", string(debug.Stack()))
	}

	return nil
}

// CheckWait check's if the condition is satisfied
func (k *Kubectl) CheckWait(namespace string, requiredStatus string, resourceName string) (bool, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "wait", "--for=condition="+requiredStatus, resourceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Error from server (NotFound)") {
			return false, nil
		}
		return false, errors.Wrapf(err, string(out))
	}
	return true, nil
}

// WaitLabelFilter waits for the condition on the resource based on label using kubectl command
func (k *Kubectl) WaitLabelFilter(namespace string, requiredStatus string, resourceName string, labelName string) error {
	if requiredStatus == "complete" {
		return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
			return k.CheckPodCompleteLabelFilter(namespace, labelName)
		})
	} else if requiredStatus == "terminate" {
		return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
			return k.CheckPodTerminateLabelFilter(namespace, labelName)
		})
	} else if requiredStatus == "ready" {
		return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
			return k.CheckPodReadyLabelFilter(namespace, resourceName, labelName, requiredStatus)
		})
	}
	return nil
}

// CheckPodReadyLabelFilter checks is the pod status is completed
func (k *Kubectl) CheckPodReadyLabelFilter(namespace string, resourceName string, labelName string, requiredStatus string) (bool, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "wait", resourceName, "-l", labelName, "--for=condition="+requiredStatus)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "no matching resources found") {
			return false, nil
		}
		return false, errors.Wrapf(err, string(out))
	}
	return true, nil
}

// CheckPodCompleteLabelFilter checks is the pod status is completed
func (k *Kubectl) CheckPodCompleteLabelFilter(namespace string, labelName string) (bool, error) {
	exitCodeTemplate := "go-template=\"{{(index (index .items 0).status.containerStatuses 0).state.terminated.exitCode}}\""
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "pod", "-l", labelName, "-o", exitCodeTemplate)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	if string(out) == "\"0\"" {
		return true, nil
	}
	return false, nil
}

// CheckPodTerminateLabelFilter checks is the pod status is terminated
func (k *Kubectl) CheckPodTerminateLabelFilter(namespace string, labelName string) (bool, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "pod", "-l", labelName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, errors.Wrapf(err, string(out))
	}
	if string(out) == "No resources found.\n" {
		return true, nil
	}
	return false, nil
}

// Delete creates the resource using kubectl command
func (k *Kubectl) Delete(namespace string, yamlFilePath string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "delete", "-f", yamlFilePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// DeleteResource deletes the resource using kubectl command
func (k *Kubectl) DeleteResource(namespace string, resourceName string, name string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "delete", resourceName, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Error from server (NotFound)") {
			return nil
		}
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// DeleteLabelFilter deletes the resource based on label using kubectl command
func (k *Kubectl) DeleteLabelFilter(namespace string, resourceName string, labelName string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "delete", resourceName, "-l", labelName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	return nil
}

// SecretCheckData checks the field specified in the given field
func (k *Kubectl) SecretCheckData(namespace string, secretName string, fieldPath string) error {
	fetchCommand := "go-template=\"{{" + fieldPath + "}}\""
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "secret", secretName, "-o", fetchCommand)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, string(out))
	}
	if len(string(out)) > 0 {
		return nil
	}
	return nil
}
