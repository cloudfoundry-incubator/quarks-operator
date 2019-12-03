package statefulset_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStatefulSet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StatefulSet Suite")
}
