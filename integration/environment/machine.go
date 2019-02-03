package environment

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
)

// Machine produces and destroys resources for tests
type Machine struct {
	pollTimeout  time.Duration
	pollInterval time.Duration

	Clientset          *kubernetes.Clientset
	VersionedClientset *versioned.Clientset
}

// TearDownFunc tears down the resource
type TearDownFunc func()

// CreatePod creates a default pod and returns a function to delete it
func (m *Machine) CreatePod(namespace string, pod corev1.Pod) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Pods(namespace)
	_, err := client.Create(&pod)
	return func() {
		client.Delete(pod.GetName(), &metav1.DeleteOptions{})
	}, err
}

// WaitForPod blocks until the pod is running. It fails after the timeout.
func (m *Machine) WaitForPod(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodRunning(namespace, name)
	})
}

// WaitForPods blocks until all selected pods are running. It fails after the timeout.
func (m *Machine) WaitForPods(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodsRunning(namespace, labels)
	})
}

// WaitForExtendedStatefulSets blocks until at least one WaitForExtendedStatefulSet is found. It fails after the timeout.
func (m *Machine) WaitForExtendedStatefulSets(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.ExtendedStatefulSetExists(namespace, labels)
	})
}

// WaitForExtendedStatefulSetAvailable blocks until latest version is available. It fails after the timeout.
func (m *Machine) WaitForExtendedStatefulSetAvailable(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.ExtendedStatefulSetAvailable(namespace, name)
	})
}

// ExtendedStatefulSetExists returns true if at least one ess selected by labels exists
func (m *Machine) ExtendedStatefulSetExists(namespace string, labels string) (bool, error) {
	esss, err := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for ess by labels: %v", labels)
	}

	return len(esss.Items) > 0, nil
}

// ExtendedStatefulSetAvailable returns true if at least one latest version pod is running
func (m *Machine) ExtendedStatefulSetAvailable(namespace string, name string) (bool, error) {
	fieldSelector := fields.Set{"metadata.name": name}.AsSelector()
	esss, err := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace).List(metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for ess by name: %v", name)
	}

	if len(esss.Items) != 1 {
		return false, nil
	}

	ess := esss.Items[0]

	if len(ess.Status.Versions) == 0 {
		return false, nil
	}

	var latestVersion int
	for latestVersion = range ess.Status.Versions {
		break
	}
	for n := range ess.Status.Versions {
		if n > latestVersion {
			latestVersion = n
		}
	}

	if ess.Status.Versions[latestVersion] {
		return true, nil
	}

	return false, nil
}

// WaitForPodsDelete blocks until the pod is deleted. It fails after the timeout.
func (m *Machine) WaitForPodsDelete(namespace string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodsDeleted(namespace)
	})
}

// PodsDeleted returns true if the all pods are deleted
func (m *Machine) PodsDeleted(namespace string) (bool, error) {
	podList, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	if len(podList.Items) == 0 {
		return true, nil
	}
	return false, nil
}

// PodRunning returns true if the pod by that name is in state running
func (m *Machine) PodRunning(namespace string, name string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pod by name: %s", name)
	}

	if pod.Status.Phase == corev1.PodRunning {
		return true, nil
	}
	return false, nil
}

// PodsRunning returns true if all the pods selected by labels are in state running
// Note that only the first page of pods is considered - don't use this if you have a
// long pod list that you care about
func (m *Machine) PodsRunning(namespace string, labels string) (bool, error) {
	pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for pod by labels: %v", labels)
	}

	if len(pods.Items) == 0 {
		return false, nil
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}
	}

	return true, nil
}

// GetPods returns all the pods selected by labels
func (m *Machine) GetPods(namespace string, labels string) (*corev1.PodList, error) {
	pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return &corev1.PodList{}, errors.Wrapf(err, "failed to query for pod by labels: %v", labels)
	}

	return pods, nil
}

// WaitForBOSHDeploymentDeletion blocks until the CR is deleted
func (m *Machine) WaitForBOSHDeploymentDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.HasBOSHDeployment(namespace, name)
		return !found, err
	})
}

// HasBOSHDeployment returns true if the pod by that name is in state running
func (m *Machine) HasBOSHDeployment(namespace string, name string) (bool, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	_, err := client.Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pod by name: %s", name)
	}

	return true, nil
}

// CreateConfigMap creates a ConfigMap and returns a function to delete it
func (m *Machine) CreateConfigMap(namespace string, configMap corev1.ConfigMap) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	_, err := client.Create(&configMap)
	return func() {
		client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
	}, err
}

// CreateSecret creates a secret and returns a function to delete it
func (m *Machine) CreateSecret(namespace string, secret corev1.Secret) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	_, err := client.Create(&secret)
	return func() {
		client.Delete(secret.GetName(), &metav1.DeleteOptions{})
	}, err
}

// UpdateConfigMap updates a ConfigMap and returns a function to delete it
func (m *Machine) UpdateConfigMap(namespace string, configMap corev1.ConfigMap) (*corev1.ConfigMap, TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	cm, err := client.Update(&configMap)
	return cm, func() {
		client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
	}, err
}

// UpdateSecret updates a secret and returns a function to delete it
func (m *Machine) UpdateSecret(namespace string, secret corev1.Secret) (*corev1.Secret, TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	s, err := client.Update(&secret)
	return s, func() {
		client.Delete(secret.GetName(), &metav1.DeleteOptions{})
	}, err
}

// GetSecret fetches the specified secret
func (m *Machine) GetSecret(namespace string, name string) (*corev1.Secret, error) {
	err := m.WaitForSecret(namespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "Waiting for secret "+name)
	}

	secret, err := m.Clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Waiting for secret "+name)
	}

	return secret, nil
}

// WaitForSecret blocks until the secret is available. It fails after the timeout.
func (m *Machine) WaitForSecret(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.SecretExists(namespace, name)
	})
}

// WaitForSecretDeletion blocks until the CR is deleted
func (m *Machine) WaitForSecretDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.SecretExists(namespace, name)
		return !found, err
	})
}

// SecretExists returns true if the pod by that name is in state running
func (m *Machine) SecretExists(namespace string, name string) (bool, error) {
	_, err := m.Clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for secret by name: %s", name)
	}

	return true, nil
}

// CreateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) CreateBOSHDeployment(namespace string, deployment bdcv1.BOSHDeployment) (*bdcv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	d, err := client.Create(&deployment)
	return d, func() {
		client.Delete(deployment.GetName(), &metav1.DeleteOptions{})
	}, err
}

// UpdateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) UpdateBOSHDeployment(namespace string, deployment bdcv1.BOSHDeployment) (*bdcv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	d, err := client.Update(&deployment)
	return d, func() {
		client.Delete(deployment.GetName(), &metav1.DeleteOptions{})
	}, err
}

// DeleteBOSHDeployment deletes a BOSHDeployment custom resource
func (m *Machine) DeleteBOSHDeployment(namespace string, name string) error {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// CreateExtendedSecret creates a ExtendedSecret custom resource and returns a function to delete it
func (m *Machine) CreateExtendedSecret(namespace string, es esv1.ExtendedSecret) (*esv1.ExtendedSecret, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedsecretV1alpha1().ExtendedSecrets(namespace)
	d, err := client.Create(&es)
	return d, func() {
		client.Delete(es.GetName(), &metav1.DeleteOptions{})
	}, err
}

// DeleteExtendedSecret deletes an ExtendedSecret custom resource
func (m *Machine) DeleteExtendedSecret(namespace string, name string) error {
	client := m.VersionedClientset.ExtendedsecretV1alpha1().ExtendedSecrets(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// CreateExtendedStatefulSet creates a ExtendedStatefulSet custom resource and returns a function to delete it
func (m *Machine) CreateExtendedStatefulSet(namespace string, ess essv1.ExtendedStatefulSet) (*essv1.ExtendedStatefulSet, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Create(&ess)
	return d, func() {
		client.Delete(ess.GetName(), &metav1.DeleteOptions{})
	}, err
}

// GetExtendedStatefulSet gets a ExtendedStatefulSet custom resource
func (m *Machine) GetExtendedStatefulSet(namespace string, name string) (*essv1.ExtendedStatefulSet, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// UpdateExtendedStatefulSet creates a ExtendedStatefulSet custom resource and returns a function to delete it
func (m *Machine) UpdateExtendedStatefulSet(namespace string, ess essv1.ExtendedStatefulSet) (*essv1.ExtendedStatefulSet, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Update(&ess)
	return d, func() {
		client.Delete(ess.GetName(), &metav1.DeleteOptions{})
	}, err
}

// DeleteExtendedStatefulSet deletes a ExtendedStatefulSet custom resource
func (m *Machine) DeleteExtendedStatefulSet(namespace string, name string) error {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// PodLabeled returns true if the pod is labeled correctly
func (m *Machine) PodLabeled(namespace string, name string, desiredLabel, desiredValue string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, err
		}
		return false, errors.Wrapf(err, "Failed to query for pod by name: %s", name)
	}

	if pod.ObjectMeta.Labels[desiredLabel] == desiredValue {
		return true, nil
	}
	return false, fmt.Errorf("Cannot match the desired label with %s", desiredValue)
}

// DeleteJobs deletes all the jobs
func (m *Machine) DeleteJobs(namespace string, labels string) (bool, error) {
	err := m.Clientset.BatchV1().Jobs(namespace).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: labels},
	)
	if err != nil {
		return false, errors.Wrapf(err, "failed to delete all jobs with labels: %s", labels)
	}

	return true, nil
}

// WaitForJobsDeleted waits until the jobs no longer exists
func (m *Machine) WaitForJobsDeleted(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to list jobs by label: %s", labels)
		}

		return len(jobs.Items) < 1, nil
	})
}

// JobExists returns true if job with that name exists
func (m *Machine) JobExists(namespace string, name string) (bool, error) {
	_, err := m.Clientset.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for job by name: %s", name)
	}

	return true, nil
}

// CollectJobs waits for n jobs with specified labels.
// It fails after the timeout.
func (m *Machine) CollectJobs(namespace string, labels string, n int) ([]batchv1.Job, error) {
	found := map[string]batchv1.Job{}
	err := wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for jobs by label: %s", labels)
		}

		for _, job := range jobs.Items {
			found[job.GetName()] = job
		}
		return len(found) >= n, nil
	})

	if err != nil {
		return nil, err
	}

	jobs := []batchv1.Job{}
	for _, job := range found {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// WaitForJobExists polls until a short timeout is reached or a job is found
// It returns true only if a job is found
func (m *Machine) WaitForJobExists(namespace string, labels string) (bool, error) {
	found := false
	err := wait.Poll(5*time.Second, 1*time.Second, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for jobs by label: %s", labels)
		}

		found = len(jobs.Items) != 0
		return found, err
	})

	if err != nil && strings.Contains(err.Error(), "timed out waiting for the condition") {
		err = nil
	}

	return found, err
}

// ContainJob searches job array for a job matching `name`
func (m *Machine) ContainJob(jobs []batchv1.Job, name string) bool {
	for _, job := range jobs {
		if strings.Contains(job.GetName(), name) {
			return true
		}
	}
	return false
}

// ContainExpectedEvent return true if events contain target resource event
func (m *Machine) ContainExpectedEvent(events []corev1.Event, reason string, message string) bool {
	for _, event := range events {
		if event.Reason == reason && strings.Contains(event.Message, message) {
			return true
		}
	}

	return false
}

// GetBOSHDeploymentEvents gets target resource events
func (m *Machine) GetBOSHDeploymentEvents(namespace string, name string, id string) ([]corev1.Event, error) {
	fieldSelector := fields.Set{"involvedObject.name": name, "involvedObject.uid": id}.AsSelector().String()
	err := m.WaitForBOSHDeploymentEvent(namespace, fieldSelector)
	if err != nil {
		return []corev1.Event{}, err
	}

	events := m.Clientset.CoreV1().Events(namespace)

	list, err := events.List(metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return []corev1.Event{}, err
	}
	return list.Items, nil
}

// WaitForBOSHDeploymentEvent gets desired event
func (m *Machine) WaitForBOSHDeploymentEvent(namespace string, fieldSelector string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.HasBOSHDeploymentEvent(namespace, fieldSelector)
		return found, err
	})
}

// HasBOSHDeploymentEvent returns true if the pod by that name is in state running
func (m *Machine) HasBOSHDeploymentEvent(namespace string, fieldSelector string) (bool, error) {
	events := m.Clientset.CoreV1().Events(namespace)
	eventList, err := events.List(metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return false, err
	}
	if len(eventList.Items) == 0 {
		return false, nil
	}
	return true, nil
}

// GetStatefulSet gets a StatefulSet custom resource
func (m *Machine) GetStatefulSet(namespace string, name string) (*v1beta1.StatefulSet, error) {
	configMap, err := m.Clientset.AppsV1beta1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return &v1beta1.StatefulSet{}, errors.Wrapf(err, "failed to query for configMap by name: %v", name)
	}

	return configMap, nil
}

// GetConfigMap gets a ConfigMap by name
func (m *Machine) GetConfigMap(namespace string, name string) (*corev1.ConfigMap, error) {
	configMap, err := m.Clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return &corev1.ConfigMap{}, errors.Wrapf(err, "failed to query for configMap by name: %v", name)
	}

	return configMap, nil
}

// GetExtendedJob gets an ExtendedJob custom resource
func (m *Machine) GetExtendedJob(namespace string, name string) (*ejv1.ExtendedJob, error) {
	client := m.VersionedClientset.Extendedjob().ExtendedJobs(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// CreateExtendedJob creates an ExtendedJob
func (m *Machine) CreateExtendedJob(namespace string, job ejv1.ExtendedJob) (*ejv1.ExtendedJob, TearDownFunc, error) {
	client := m.VersionedClientset.Extendedjob().ExtendedJobs(namespace)
	d, err := client.Create(&job)
	return d, func() {
		client.Delete(job.GetName(), &metav1.DeleteOptions{})
	}, err
}

// UpdateExtendedJob updates an extended job
func (m *Machine) UpdateExtendedJob(namespace string, exJob ejv1.ExtendedJob) error {
	client := m.VersionedClientset.Extendedjob().ExtendedJobs(namespace)
	_, err := client.Update(&exJob)
	return err
}
