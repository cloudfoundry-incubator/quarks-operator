package machinery

import (
	fissile "code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/client/clientset/versioned"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Machine produces and destroys resources for tests
type Machine struct {
	Clientset          *kubernetes.Clientset
	VersionedClientset *versioned.Clientset
}

// TearDownFunc tears down the resource
type TearDownFunc func()

// CreateConfigMap creates a ConfigMap and returns a function to delete it
func (m *Machine) CreateConfigMap(namespace string, configMap apiv1.ConfigMap) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	_, err := client.Create(&configMap)
	return func() {
		client.Delete(configMap.GetName(), &v1.DeleteOptions{})
	}, err
}

// CreateSecret creates a secret and returns a function to delete it
func (m *Machine) CreateSecret(namespace string, secret apiv1.Secret) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	_, err := client.Create(&secret)
	return func() {
		client.Delete(secret.GetName(), &v1.DeleteOptions{})
	}, err
}

// CreateBOSHDeployment creates a BOSHDeployment and returns a function to delete it
func (m *Machine) CreateBOSHDeployment(namespace string, deployment fissile.BOSHDeployment) (TearDownFunc, error) {
	client := m.VersionedClientset.Fissile().BOSHDeployments(namespace)
	_, err := client.Create(&deployment)
	return func() {
		client.Delete(deployment.GetName(), &v1.DeleteOptions{})
	}, err
}

// DefaultConfigMap for tests
func (m *Machine) DefaultConfigMap(name string) apiv1.ConfigMap {
	return apiv1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": `instance-groups:
- name: diego
  instances: 3
- name: mysql
`,
		},
	}
}

// DefaultSecret for tests
func (m *Machine) DefaultSecret(name string) apiv1.Secret {
	return apiv1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: name},
		StringData: map[string]string{},
	}
}

// DefaultBOSHDeployment for tests
func (m *Machine) DefaultBOSHDeployment(name, manifestRef string) fissile.BOSHDeployment {
	return fissile.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fissile.BOSHDeploymentSpec{
			ManifestRef: manifestRef,
		},
	}
}
