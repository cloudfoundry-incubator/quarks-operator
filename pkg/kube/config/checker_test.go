package config

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var _ = Describe("Checker", func() {
	Describe("NewDefaultChecker constructor", func() {
		It("returns a checker", func() {
			c := NewChecker(nil)
			_, ok := c.(*checker)
			Expect(ok).To(Equal(true))
		})
	})

	type checkCase struct {
		checker checker

		cfg         *rest.Config
		expectedErr error
	}

	DescribeTable(
		"Check method",
		func(c checkCase) {
			logger := zap.NewNop()
			defer logger.Sync()
			c.checker.log = logger.Sugar()

			actualErr := c.checker.Check(c.cfg)
			if c.expectedErr == nil {
				Expect(actualErr).To(BeNil())
			} else {
				Expect(actualErr).To(Equal(c.expectedErr))
			}
		},
		Entry(
			"should fail when creating the k8s clientset fails",
			checkCase{
				checker: checker{
					createClientSet: func(c *rest.Config) (*kubernetes.Clientset, error) {
						return nil, fmt.Errorf("error from createClientSet")
					},
				},
				expectedErr: &checkConfigError{fmt.Errorf("error from createClientSet")},
			},
		),
		Entry(
			"should fail when checking the server version fails",
			checkCase{
				checker: checker{
					createClientSet: func(c *rest.Config) (*kubernetes.Clientset, error) {
						return &kubernetes.Clientset{}, nil
					},
					checkServerVersion: func(d discovery.ServerVersionInterface) error {
						return fmt.Errorf("error from checkServerVersion")
					},
				},
				expectedErr: &checkConfigError{fmt.Errorf("error from checkServerVersion")},
			},
		),
		Entry(
			"should succeed with no errors",
			checkCase{
				checker: checker{
					createClientSet: func(c *rest.Config) (*kubernetes.Clientset, error) {
						return &kubernetes.Clientset{}, nil
					},
					checkServerVersion: func(d discovery.ServerVersionInterface) error {
						return nil
					},
				},
			},
		),
	)

	Describe("checkConfigError", func() {
		It("Error() should construct the error correctly", func() {
			err := checkConfigError{fmt.Errorf("some error")}
			Expect(err.Error()).To(Equal(fmt.Sprintf("invalid kube config: some error")))
		})
	})

	Describe("checkServerVersion", func() {
		It("should call the ServerVersion method and return only the error from it", func() {
			expectedErr := fmt.Errorf("error from ServerVersion")
			actualErr := checkServerVersion(&discoveryMock{expectedErr})
			Expect(actualErr).To(Equal(expectedErr))
		})
	})
})

type discoveryMock struct {
	err error
}

func (d *discoveryMock) ServerVersion() (*version.Info, error) {
	return nil, d.err
}
