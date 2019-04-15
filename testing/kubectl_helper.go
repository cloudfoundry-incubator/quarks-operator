package testing

import (
	"bytes"
	"os/exec"
	"strings"
	"time"

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
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// DeleteNamespace deletes the namespace using kubectl command
func (k *Kubectl) DeleteNamespace(name string) error {
	cmd := exec.Command("kubectl", "delete", "namespace", name)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Create creates the resource using kubectl command
func (k *Kubectl) Create(namespace string, yamlFilePath string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "create", "-f", yamlFilePath)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Apply updates the resource using kubectl command
func (k *Kubectl) Apply(namespace string, yamlFilePath string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "apply", "-f", yamlFilePath)
	err := cmd.Run()
	if err != nil {
		return err
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
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	if len(out.String()) > 0 {
		return out.String(), nil
	}
	return "", err
}

// GetSecretData fetches the specified output by the given templatePath
func (k *Kubectl) GetSecretData(namespace string, secretName string, templatePath string) ([]byte, error) {
	out, err := exec.Command("kubectl", "--namespace", namespace, "get", "secret", secretName, "-o", templatePath).Output()
	if err != nil {
		return []byte{}, err
	}
	if len(string(out)) > 0 {
		return out, nil
	}
	return []byte{}, err
}

// Wait waits for the condition on the resource using kubectl command
func (k *Kubectl) Wait(namespace string, requiredStatus string, resourceName string) error {
	return wait.PollImmediate(k.pollInterval, k.pollTimeout, func() (bool, error) {
		return k.CheckWait(namespace, requiredStatus, resourceName)
	})
}

// CheckWait check's if the condition is satisfied
func (k *Kubectl) CheckWait(namespace string, requiredStatus string, resourceName string) (bool, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "wait", "--for=condition="+requiredStatus, resourceName)
	err := cmd.Run()
	if err != nil {
		return false, nil
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
	err := cmd.Run()
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CheckPodCompleteLabelFilter checks is the pod status is completed
func (k *Kubectl) CheckPodCompleteLabelFilter(namespace string, labelName string) (bool, error) {
	exitCodeTemplate := "go-template=\"{{(index (index .items 0).status.containerStatuses 0).state.terminated.exitCode}}\""
	out, err := exec.Command("kubectl", "--namespace", namespace, "get", "pod", "-l", labelName, "-o", exitCodeTemplate).Output()
	if err != nil {
		return false, nil
	}
	if string(out) == "\"0\"" {
		return true, nil
	} else {
		return false, nil
	}
}

// CheckPodTerminateLabelFilter checks is the pod status is completed
func (k *Kubectl) CheckPodTerminateLabelFilter(namespace string, labelName string) (bool, error) {
	out, err := exec.Command("kubectl", "--namespace", namespace, "get", "pod", "-l", labelName).Output()
	if err != nil {
		return false, err
	}
	if string(out) == "" {
		return true, nil
	}
	return false, nil
}

// Delete creates the resource using kubectl command
func (k *Kubectl) Delete(namespace string, yamlFilePath string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "delete", "-f", yamlFilePath)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// DeleteResource deletes the resource using kubectl command
func (k *Kubectl) DeleteResource(namespace string, resourceName string, name string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "delete", resourceName, name)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// DeleteLabelFilter deletes the resource based on label using kubectl command
func (k *Kubectl) DeleteLabelFilter(namespace string, resourceName string, labelName string) error {
	cmd := exec.Command("kubectl", "--namespace", namespace, "delete", resourceName, "-l", labelName)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// SecretCheckData checks the field specified in the given field
func (k *Kubectl) SecretCheckData(namespace string, secretName string, fieldPath string) error {
	fetchCommand := "go-template=\"{{" + fieldPath + "}}\""
	out, err := exec.Command("kubectl", "--namespace", namespace, "get", "secret", secretName, "-o", fetchCommand).Output()
	if err != nil {
		return err
	}
	if len(string(out)) > 0 {
		return nil
	}
	return nil
}
