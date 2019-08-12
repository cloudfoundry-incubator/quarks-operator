package fakes

import (
	"fmt"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// FakeClientConfig is a mock of Client ClientConfig
type FakeClientConfig struct {
	ExpectedClientConfigError bool
}

// RawConfig mocks base method
func (f *FakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, nil
}

// ClientConfig mocks base method
func (f *FakeClientConfig) ClientConfig() (*restclient.Config, error) {
	if f.ExpectedClientConfigError {
		return nil, fmt.Errorf("error from ClientConfig")
	}
	return &restclient.Config{Host: "another.cluster.config.com"}, nil
}

// Namespace mocks base method
func (f *FakeClientConfig) Namespace() (string, bool, error) {
	return "", false, nil
}

// ConfigAccess mocks base method
func (f *FakeClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return &clientcmd.ClientConfigLoadingRules{}
}
