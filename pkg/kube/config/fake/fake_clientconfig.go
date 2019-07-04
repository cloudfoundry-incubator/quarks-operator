package fake

import (
	"fmt"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type FakeClientConfig struct {
	ExpectedClientConfigError bool
}

func (f *FakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, nil
}

func (f *FakeClientConfig) ClientConfig() (*restclient.Config, error) {
	if f.ExpectedClientConfigError {
		return nil, fmt.Errorf("error from ClientConfig")
	}
	return &restclient.Config{Host: "another.cluster.config.com"}, nil
}

func (f *FakeClientConfig) Namespace() (string, bool, error) {
	return "", false, nil
}

func (f *FakeClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return &clientcmd.ClientConfigLoadingRules{}
}
