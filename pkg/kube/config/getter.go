package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Getter is the interface that wraps the Get method that returns the Kubernetes configuration used
// to communicate with it using its API.
type Getter interface {
	Get(customConfigPath string) (*rest.Config, error)
}

// NewGetter constructs a default getter that satisfies the Getter interface.
func NewGetter(log *zap.SugaredLogger) Getter {
	return &getter{
		log: log,

		inClusterConfig:          rest.InClusterConfig,
		lookupEnv:                os.LookupEnv,
		readFile:                 ioutil.ReadFile,
		restConfigFromKubeConfig: clientcmd.RESTConfigFromKubeConfig,
		currentUser:              user.Current,
		defaultRESTConfig:        clientcmd.DefaultClientConfig.ClientConfig,
	}
}

type getter struct {
	log *zap.SugaredLogger

	inClusterConfig          func() (*rest.Config, error)
	lookupEnv                func(key string) (string, bool)
	readFile                 func(filename string) ([]byte, error)
	restConfigFromKubeConfig func(configBytes []byte) (*rest.Config, error)
	currentUser              func() (*user.User, error)
	defaultRESTConfig        func() (*rest.Config, error)
}

func (g *getter) Get(customConfigPath string) (*rest.Config, error) {
	configPath := customConfigPath

	if configPath == "" {
		// If no explicit location, try the in-cluster config.
		_, okHost := g.lookupEnv("KUBERNETES_SERVICE_HOST")
		_, okPort := g.lookupEnv("KUBERNETES_SERVICE_PORT")
		if okHost && okPort {
			c, err := g.inClusterConfig()
			if err == nil {
				g.log.Info("Using in-cluster kube config")
				return c, nil
			} else if !os.IsNotExist(err) {
				return nil, &getConfigError{err}
			}
		}

		// If no in-cluster config, set the config path to the user's ~/.kube directory.
		usr, err := g.currentUser()
		if err != nil {
			return nil, &getConfigError{err}
		}
		configPath = filepath.Join(usr.HomeDir, ".kube", "config")
	}

	b, err := g.readFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, &getConfigError{err}
		}

		// If neither the custom config path, nor the user's ~/.kube directory config path exist, use a
		// default config.
		c, err := g.defaultRESTConfig()
		if err != nil {
			return nil, &getConfigError{err}
		}
		g.log.Infof("%s does not exist, using default kube config", configPath)
		return c, nil
	}

	c, err := g.restConfigFromKubeConfig(b)
	if err != nil {
		return nil, &getConfigError{err}
	}
	g.log.Infof("Using kube config '%s'", configPath)
	return c, nil
}

type getConfigError struct {
	err error
}

func (e *getConfigError) Error() string {
	return fmt.Sprintf("failed to get kube config: %v", e.err)
}
