// +build tools

// Dummy package to pin kubernetes/code-generator
// from go.mod used by 'make kube-gen'.
// Required to allow "go mod vendor" to fetch the same kubecodegen
// version pinned in quarks-utils

package codegen

import _ "code.cloudfoundry.org/quarks-utils/pkg/kubecodegen"
