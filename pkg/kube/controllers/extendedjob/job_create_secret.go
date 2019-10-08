package extendedjob

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/fsnotify.v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var (
	// LabelEjobPod is the label present on every jobpod of exjob.
	LabelEjobPod = fmt.Sprintf("%s/ejob-name", apis.GroupName)
)

// PersistOutputInterface creates a kubernetes secret for each container in the in the extendedjob pod.
type PersistOutputInterface struct {
	namespace            string
	podName              string
	clientSet            kubernetes.Interface
	versionedClientSet   versioned.Interface
	outputFilePathPrefix string
}

// NewPersistOutputInterface returns a persistoutput interface which can create kubernetes secrets.
func NewPersistOutputInterface(namespace string, podName string, clientSet kubernetes.Interface, versionedClientSet versioned.Interface, outputFilePathPrefix string) *PersistOutputInterface {
	return &PersistOutputInterface{
		namespace:            namespace,
		podName:              podName,
		clientSet:            clientSet,
		versionedClientSet:   versionedClientSet,
		outputFilePathPrefix: outputFilePathPrefix,
	}
}

// PersistOutput converts the output files of each container
// in the pod related to an ejob into a kubernetes secret.
func (po *PersistOutputInterface) PersistOutput() error {

	// Fetch the pod and job
	pod, err := po.clientSet.CoreV1().Pods(po.namespace).Get(po.podName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch pod %s", po.podName)
	}

	// Fetch the exjob and job
	exjobName := pod.GetLabels()[LabelEjobPod]

	exjobClient := po.versionedClientSet.ExtendedjobV1alpha1().ExtendedJobs(po.namespace)
	exjob, err := exjobClient.Get(exjobName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch exjob")
	}

	// Persist output if needed
	if !reflect.DeepEqual(ejv1.Output{}, exjob.Spec.Output) && exjob.Spec.Output != nil {
		err = po.ConvertOutputToSecretPod(pod, exjob)
		if err != nil {
			return err
		}
	}
	return nil
}

// ConvertOutputToSecretPod starts goroutine for converting each container
// output into a secret.
func (po *PersistOutputInterface) ConvertOutputToSecretPod(pod *corev1.Pod, exjob *ejv1.ExtendedJob) error {

	//fileNotifyChannel := make(chan int)
	//errorChannel := make(chan error)

	// Loop over containers
	for _, container := range pod.Spec.Containers {

		if container.Name == "output-persist" {
			continue
		}

		go po.ConvertOutputToSecretContainer(container)

	}

	/*// wait for all the go routines
	for i := 0; i < len(pod.Spec.Containers)-1; i++ {
		select {
		case containerIndex := <-fileNotifyChannel:
			getContainerState()
			// Fetch the pod and job
			pod, err := po.clientSet.CoreV1().Pods(po.namespace).Get(po.podName, metav1.GetOptions{})
			if err != nil {
				return errors.Wrapf(err, "failed to fetch pod %s", po.podName)
			}
			outputContainer := pod.Spec.Containers[containerIndex]
			fmt.Println("Container Name", outputContainer.Name, pod.Status.ContainerStatuses[containerIndex].State.Te, pod.Status.ContainerStatuses[containerIndex].Name)
			err := po.CreateSecret(outputContainer, pod.Name, exjob)
			if err != nil {
				return err
			}
		case failure := <-errorChannel:
			return errors.Wrapf(failure, "failure waiting for output file for container in pod %s", pod.GetName())
		}
	}*/
	return nil
}

// ConvertOutputToSecretContainer converts json output file
// of the specified container into a secret
func (po *PersistOutputInterface) ConvertOutputToSecretContainer(container corev1.Container) error {
	filePath := filepath.Join(po.outputFilePathPrefix, container.Name, "output.json")
	fileFound, err := po.CheckForOutputFile(filePath, container.Name)
	if err != nil {
		return err
	}
	if fileFound {
		_, err := po.GetContainerExitCode(container.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetContainerExitCode returns the exit code of the container
func (po *PersistOutputInterface) GetContainerExitCode(containerName string) (int, error) {

	// Wait until the container state is terminated
	for {
		pod, err := po.clientSet.CoreV1().Pods(po.namespace).Get(po.podName, metav1.GetOptions{})
		if err != nil {
			return -1, errors.Wrapf(err, "failed to fetch pod %s", po.podName)
		}
		for continerStatusIndex, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name == containerName {
				fmt.Println(pod.Status.ContainerStatuses[continerStatusIndex].State.Terminated)
				break
			}
		}
	}
	//return 0, nil
}

// CheckForOutputFile waits for the output json file to be created
// in the container
func (po *PersistOutputInterface) CheckForOutputFile(filePath string, containerName string) (bool, error) {

	if fileExists(filePath) {
		return true, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return false, err
	}
	defer watcher.Close()

	createEventFileChannel := make(chan bool)
	errorEventFileChannel := make(chan error)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					continue
				}
				if event.Op == fsnotify.Create && event.Name == filePath {
					createEventFileChannel <- true
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					continue
				}
				errorEventFileChannel <- err
			}
		}
	}()

	err = watcher.Add(filepath.Join(po.outputFilePathPrefix, containerName))
	if err != nil {
		return false, err
	}

	select {
	case fileFound := <-createEventFileChannel:
		return fileFound, nil
	case err := <-errorEventFileChannel:
		return false, err
	}
}

// CreateSecret converts the output file into json and creates a secret for a given container
func (po *PersistOutputInterface) CreateSecret(outputContainer corev1.Container, podName string, exjob *ejv1.ExtendedJob) error {

	namePrefix := exjob.Spec.Output.NamePrefix
	secretName := namePrefix + outputContainer.Name

	// Fetch json from file
	filePath := filepath.Join(po.outputFilePathPrefix, outputContainer.Name, "output.json")
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrapf(err, "unable to read file %s in container %s in pod %s", filePath, outputContainer.Name, podName)
	}
	var data map[string]string
	err = json.Unmarshal([]byte(file), &data)
	if err != nil {
		return errors.Wrapf(err, "failed to convert output file %s into json for creating secret %s in pod %s",
			filePath, secretName, podName)
	}

	// Create secret for the outputfile to persist
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: po.namespace,
		},
	}
	secretLabels := exjob.Spec.Output.SecretLabels
	if secretLabels == nil {
		secretLabels = map[string]string{}
	}
	secretLabels[ejv1.LabelPersistentSecretContainer] = outputContainer.Name
	if ig, ok := podutil.LookupEnv(outputContainer.Env, converter.EnvInstanceGroupName); ok {
		secretLabels[ejv1.LabelInstanceGroup] = ig
	}

	if exjob.Spec.Output.Versioned {
		// Use secretName as versioned secret name prefix: <secretName>-v<version>
		err = po.CreateVersionSecret(exjob.GetName(), exjob.GetUID(), secretName, data, secretLabels, "created by extendedjob")
		if err != nil {
			return errors.Wrapf(err, "could not persist ejob's %s output to a secret", exjob.GetName())
		}
	} else {
		secret.StringData = data
		secret.Labels = secretLabels

		_, err = po.clientSet.CoreV1().Secrets(po.namespace).Create(secret)

		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// If it exists update it
				_, err = po.clientSet.CoreV1().Secrets(po.namespace).Update(secret)
				if err != nil {
					return errors.Wrapf(err, "failed to update secret %s for container %s in pod %s.", secretName, outputContainer.Name, podName)
				}
			} else {
				return errors.Wrapf(err, "failed to create secret %s for container %s in pod %s.", secretName, outputContainer.Name, podName)
			}
		}

	}
	return nil
}

// CreateVersionSecret create a versioned kubernetes secret given the data.
func (po *PersistOutputInterface) CreateVersionSecret(ownerName string, ownerID types.UID, secretName string, secretData map[string]string, labels map[string]string, sourceDescription string) error {
	currentVersion, err := getGreatestVersion(po.clientSet, po.namespace, secretName)
	if err != nil {
		return err
	}

	version := currentVersion + 1
	labels[versionedsecretstore.LabelVersion] = strconv.Itoa(version)
	labels[versionedsecretstore.LabelSecretKind] = versionedsecretstore.VersionSecretKind

	generatedSecretName, err := versionedsecretstore.GenerateSecretName(secretName, version)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedSecretName,
			Namespace: po.namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         versionedsecretstore.LabelAPIVersion,
					Kind:               "ExtendedJob",
					Name:               ownerName,
					UID:                ownerID,
					BlockOwnerDeletion: util.Bool(false),
					Controller:         util.Bool(true),
				},
			},
			Annotations: map[string]string{
				versionedsecretstore.AnnotationSourceDescription: sourceDescription,
			},
		},
		StringData: secretData,
	}

	_, err = po.clientSet.CoreV1().Secrets(po.namespace).Create(secret)
	return err
}

// fileExists checks if the file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getGreatestVersion(clientSet kubernetes.Interface, namespace string, secretName string) (int, error) {
	list, err := listSecrets(clientSet, namespace, secretName)
	if err != nil {
		return -1, err
	}

	var greatestVersion int
	for _, secret := range list {
		version, err := versionedsecretstore.Version(secret)
		if err != nil {
			return 0, err
		}

		if version > greatestVersion {
			greatestVersion = version
		}
	}

	return greatestVersion, nil
}

func listSecrets(clientSet kubernetes.Interface, namespace string, secretName string) ([]corev1.Secret, error) {
	secretLabelsSet := labels.Set{
		versionedsecretstore.LabelSecretKind: versionedsecretstore.VersionSecretKind,
	}

	secrets, err := clientSet.CoreV1().Secrets(namespace).List(metav1.ListOptions{
		LabelSelector: secretLabelsSet.String(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list secrets with labels %s", secretLabelsSet.String())
	}

	result := []corev1.Secret{}

	nameRegex := regexp.MustCompile(fmt.Sprintf(`^%s-v\d+$`, secretName))
	for _, secret := range secrets.Items {
		if nameRegex.MatchString(secret.Name) {
			result = append(result, secret)
		}
	}

	return result, nil
}
