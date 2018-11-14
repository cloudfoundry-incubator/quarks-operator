package integration_test

import (
	"log"
	"os"
	"path/filepath"

	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

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
	mgr         manager.Manager
	namespace   string
	kubeConfig  *rest.Config
	logRecorded *observer.ObservedLogs
	log         *zap.SugaredLogger
}

var suite = &IntegrationSuite{
	namespace: "",
}

var _ = BeforeSuite(func() {
	suite.setup()

	err := suite.startClients(suite.kubeConfig)
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

func (s *IntegrationSuite) setup() {
	ns, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		ns = "default"
	}
	s.namespace = ns

	var core zapcore.Core
	core, s.logRecorded = observer.New(zapcore.InfoLevel)
	s.log = zap.New(core).Sugar()

	err := suite.setupKube()
	Expect(err).NotTo(HaveOccurred())

	suite.mgr, err = operator.NewManager(suite.log, suite.kubeConfig, manager.Options{Namespace: suite.namespace})
	Expect(err).NotTo(HaveOccurred())
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

func (s *IntegrationSuite) startClients(kubeConfig *rest.Config) (err error) {
	s.Clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return
	}
	s.VersionedClientset, err = versioned.NewForConfig(kubeConfig)
	return
}

func (s *IntegrationSuite) startOperator() {
	stop = make(chan struct{})
	go s.mgr.Start(stop)
}
