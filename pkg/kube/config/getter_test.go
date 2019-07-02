package config

import (
	"fmt"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"
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

		customConfigPath string
		expectedConfig   *rest.Config
		expectedErr      error
	}

	DescribeTable(
		"Get method",
		func(c getCase) {
			logger := zap.NewNop()
			defer logger.Sync()
			c.getter.log = logger.Sugar()

			actualConfig, actualErr := c.getter.Get(c.customConfigPath)
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
			"should fail when reading the config from ~/.kube fails",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
				},
				expectedErr: &getConfigError{fmt.Errorf("error from readFile that isn't NotExist")},
			},
		),
		Entry(
			"should fail when creating the output rest config from ~/.kube fails",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
				},
				expectedErr: &getConfigError{fmt.Errorf("error from restConfigFromKubeConfig")},
			},
		),
		Entry(
			"should succeed when creating the output rest config from ~/.kube",
			getCase{
				getter: getter{
					lookupEnv: func(_ string) (string, bool) {
						return "", false
					},
					currentUser: func() (*user.User, error) {
						return &user.User{HomeDir: filepath.Join("home", "johndoe")}, nil
					},
				},
				expectedConfig: &rest.Config{Host: "home.kube.config.com"},
			},
		),
		Entry(
			"should fail when reading the config from the provided config path fails",
			getCase{
				getter:           getter{},
				customConfigPath: filepath.Join("path", "to", ".kube", "config"),
				expectedErr:      &getConfigError{fmt.Errorf("error from readFile that isn't NotExist")},
			},
		),
		Entry(
			"should fail when creating the output rest config from the provided config path fails",
			getCase{
				getter:           getter{},
				customConfigPath: filepath.Join("path", "to", ".kube", "config"),
				expectedErr:      &getConfigError{fmt.Errorf("error from restConfigFromKubeConfig")},
			},
		),
		Entry(
			"should succeed when creating the output rest config from the provided config path",
			getCase{
				getter:           getter{},
				customConfigPath: filepath.Join("path", "to", ".kube", "config"),
				expectedConfig:   &rest.Config{Host: "provided.kube.config.com"},
			},
		),
		Entry(
			"should fail when creating the output rest config fails using the default REST config",
			getCase{
				getter: getter{
					defaultRESTConfig: func() (*rest.Config, error) {
						return nil, fmt.Errorf("error from defaultRESTConfig")
					},
				},
				customConfigPath: filepath.Join("path", "to", ".kube", "config"),
				expectedErr:      &getConfigError{fmt.Errorf("error from defaultRESTConfig")},
			},
		),
		Entry(
			"should succeed when creating the output rest config using the default REST config",
			getCase{
				getter: getter{
					defaultRESTConfig: func() (*rest.Config, error) {
						return &rest.Config{Host: "default.rest.config.com"}, nil
					},
				},
				customConfigPath: filepath.Join("path", "to", ".kube", "config"),
				expectedConfig:   &rest.Config{Host: "default.rest.config.com"},
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
