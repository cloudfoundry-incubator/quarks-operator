package manifest

import (
	"fmt"

	"github.com/cppforlife/go-patch/patch"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Interpolator renders BOSH manifests by operations files
// go:generate counterfeiter -o fakes/fake_interpolator.go . Interpolator
type Interpolator interface {
	BuildOps(opsBytes []byte) error
	Interpolate(manifestBytes []byte) ([]byte, error)
}

// InterpolatorImpl applies desired changes from BOSH operations files to to BOSH manifest
type InterpolatorImpl struct {
	ops patch.Ops
}

// NewInterpolator constructs an interpolator
func NewInterpolator() *InterpolatorImpl {
	return &InterpolatorImpl{}
}

// BuildOps unmarshals ops definitions, processes them and holds them in memory
func (i *InterpolatorImpl) BuildOps(opsBytes []byte) error {
	var opDefs []patch.OpDefinition
	err := yaml.Unmarshal(opsBytes, &opDefs)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("deserializing ops data '%s'", opsBytes))
	}

	ops, err := patch.NewOpsFromDefinitions(opDefs)
	if err != nil {
		return errors.Wrap(err, "building ops")
	}
	i.ops = append(i.ops, ops)
	return nil
}

// Interpolate returns manifest which is rendered by operations files
func (i *InterpolatorImpl) Interpolate(manifestBytes []byte) ([]byte, error) {

	// Decode manifest
	var obj interface{}

	err := yaml.Unmarshal(manifestBytes, &obj)
	if err != nil {
		return []byte{}, err
	}

	// Apply ops
	if i.ops != nil {
		obj, err = i.ops.Apply(obj)
		if err != nil {
			return []byte{}, err
		}
	}

	// Interpolate from root
	obj, err = i.interpolateRoot(obj)
	if err != nil {
		return []byte{}, err
	}

	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

func (i *InterpolatorImpl) interpolateRoot(obj interface{}) (interface{}, error) {
	obj, err := i.interpolate(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (i *InterpolatorImpl) interpolate(node interface{}) (interface{}, error) {
	switch typedNode := node.(type) {
	case map[interface{}]interface{}:
		for k, v := range typedNode {
			evaluatedValue, err := i.interpolate(v)
			if err != nil {
				return nil, err
			}

			evaluatedKey, err := i.interpolate(k)
			if err != nil {
				return nil, err
			}

			delete(typedNode, k) // delete in case key has changed
			typedNode[evaluatedKey] = evaluatedValue
		}

	case []interface{}:
		for idx, x := range typedNode {
			var err error
			typedNode[idx], err = i.interpolate(x)
			if err != nil {
				return nil, err
			}
		}

	case string:
		return typedNode, nil
	}

	return node, nil
}
