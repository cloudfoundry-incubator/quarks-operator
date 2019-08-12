package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"code.cloudfoundry.org/cf-operator/pkg/kube/config/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero/mem"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = Describe("Getter", func() {
	Describe("NewDefaultGetter constructor", func() {
		It("returns a getter", func() {
			g := NewGetter(nil)
			_, ok := g.(*getter)
			Expect(ok).To(Equal(true))
		})
	})

	type getCase struct {
		getter getter

		configPath     string
		expectedConfig *rest.Config
		expectedErr    error
	}

	DescribeTable(
		"Get method",
		func(c getCase) {
			logger := zap.NewNop()
			defer logger.Sync()
			c.getter.log = logger.Sugar()

			actualConfig, actualErr := c.getter.Get(c.configPath)
			if c.expectedConfig == nil {
				Expect(actualConfig).To(BeNil())
			} else {
				Expect(actualConfig).To(Equal(c.expectedConfig))
			}
			if c.expectedErr == nil {
				Expect(actualErr).To(BeNil())
			} else {
				Expect(actualErr).To(Equal(c.expectedErr))
			}
		},
		Entry(
			"should fail when loading the in-cluster config fails",
			getCase{
				getter: getter{
					inClusterConfig: func() (*rest.Config, error) {
						return nil, fmt.Errorf("error from inClusterConfig")
					},
					lookupEnv: func(_ string) (string, bool) {
						return "", true
					},
				},
				expectedErr: &getConfigError{fmt.Errorf("error from inClusterConfig")},
			},
		),
		Entry(
			"should succeed when loading the in-cluster config",
			getCase{
				getter: getter{
					inClusterConfig: func() (*rest.Config, error) {
						return &rest.Config{Host: "in.cluster.config.com"}, nil
					},
					lookupEnv: func(_ string) (string, bool) {
						return "", true
					},
				},
				expectedConfig: &rest.Config{Host: "in.cluster.config.com"},
			},
		),
		Entry(
			"should fail when fetching the current user executing the program fails",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return nil, fmt.Errorf("error from currentUser")
					},
				},
				expectedErr: &getConfigError{fmt.Errorf("error from currentUser")},
			},
		),
		Entry(
			"should succeed when using the config from ~/.kube",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
					stat: func(name string) (os.FileInfo, error) {
						Expect(name).To(Equal(filepath.Join("home", "johndoe", ".kube", "config")))
						return &mem.FileInfo{}, nil
					},
					restConfigFromKubeConfig: func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
						return &fakes.FakeClientConfig{}
					},
				},
				expectedConfig: &rest.Config{Host: "another.cluster.config.com"},
			},
		),
		Entry(
			"should fail when stating the config from ~/.kube fails",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
					stat: func(name string) (os.FileInfo, error) {
						Expect(name).To(Equal(filepath.Join("home", "johndoe", ".kube", "config")))
						return &mem.FileInfo{}, fmt.Errorf("error from stat that isn't NotExist")
					},
				},
				expectedErr: &getConfigError{fmt.Errorf("error from stat that isn't NotExist")},
			},
		),
		Entry(
			"should succeed when creating the output rest config using the default REST config",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
					stat: func(filename string) (os.FileInfo, error) {
						Expect(filename).To(Equal(filepath.Join("home", "johndoe", ".kube", "config")))
						return &mem.FileInfo{}, os.ErrNotExist
					},
					defaultRESTConfig: func() (*rest.Config, error) {
						return &rest.Config{Host: "default.rest.config.com"}, nil
					},
				},
				expectedConfig: &rest.Config{Host: "default.rest.config.com"},
			},
		),
		Entry(
			"should fail when using the default REST config fails",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
					stat: func(filename string) (os.FileInfo, error) {
						Expect(filename).To(Equal(filepath.Join("home", "johndoe", ".kube", "config")))
						return &mem.FileInfo{}, os.ErrNotExist
					},
					defaultRESTConfig: func() (*rest.Config, error) {
						return &rest.Config{}, fmt.Errorf("error from defaultRESTConfig")
					},
				}, expectedErr: &getConfigError{fmt.Errorf("error from defaultRESTConfig")},
			},
		),
		Entry(
			"should succeed when using the config from configPath",
			getCase{
				configPath: "/config/path",
				getter: getter{
					restConfigFromKubeConfig: func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
						Expect(loader.GetExplicitFile()).To(Equal("/config/path"))
						return &fakes.FakeClientConfig{}
					},
				},
				expectedConfig: &rest.Config{Host: "another.cluster.config.com"},
			},
		),
		Entry(
			"should succeed when using the config from configPath which has multi configs",
			getCase{
				configPath: fmt.Sprintf("/config/path1%s/config/path2", string(os.PathListSeparator)),
				getter: getter{
					restConfigFromKubeConfig: func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
						Expect(loader.GetLoadingPrecedence()).To(Equal([]string{"/config/path1", "/config/path2"}))
						Expect(loader.GetExplicitFile()).To(BeEmpty())
						return &fakes.FakeClientConfig{}
					},
				},
				expectedConfig: &rest.Config{Host: "another.cluster.config.com"},
			},
		),
		Entry(
			"should fail when getting ClientConfig from configPath fails",
			getCase{
				configPath: "/config/path",
				getter: getter{
					restConfigFromKubeConfig: func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
						return &fakes.FakeClientConfig{
							ExpectedClientConfigError: true,
						}
					},
				},
				expectedErr: &getConfigError{fmt.Errorf("error from ClientConfig")},
			},
		),
	)

	Describe("getConfigError", func() {
		It("Error() should construct the error correctly", func() {
			err := getConfigError{fmt.Errorf("some error")}
			Expect(err.Error()).To(Equal(fmt.Sprintf("failed to get kube config: some error")))
		})
	})
})
