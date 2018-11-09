package integration_test

import (
	"log"
	"os"
	"path/filepath"

	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/integration/machinery"
	"code.cloudfoundry.org/cf-operator/pkg/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/operator"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var stop chan struct{}

type IntegrationSuite struct {
	machinery.Machine
	mgr        manager.Manager
	namespace  string
	kubeConfig *rest.Config
}

var suite = &IntegrationSuite{
	namespace: "",
}

var _ = BeforeSuite(func() {
	err := suite.setupKube()
	Expect(err).NotTo(HaveOccurred())

	suite.namespace = suite.getTestNamespace()
	suite.mgr, err = operator.Setup(suite.kubeConfig, manager.Options{Namespace: suite.namespace})
	Expect(err).NotTo(HaveOccurred())

	suite.startClients(suite.kubeConfig)
	Expect(err).NotTo(HaveOccurred())

	suite.startOperator()
})

var _ = AfterSuite(func() {
	defer func() {
		if stop != nil {
			close(stop)
		}
	}()
})

func (s *IntegrationSuite) startClients(kubeConfig *rest.Config) (err error) {
	s.Clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return
	}
	s.VersionedClientset, err = versioned.NewForConfig(kubeConfig)
	return
}

func (s *IntegrationSuite) getTestNamespace() string {
	ns, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		return "default"
	}
	return ns
}

func (s *IntegrationSuite) setupKube() (err error) {
	location := os.Getenv("KUBE_CONFIG")
	if location == "" {
		location = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	s.kubeConfig, err = clientcmd.BuildConfigFromFlags("", location)
	if err != nil {
		log.Printf("INFO: cannot use kube config: %s\n", err)
		s.kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			return
		}
	}

	return
}

func (s *IntegrationSuite) startOperator() {
	stop = make(chan struct{})
	go s.mgr.Start(stop)
}
