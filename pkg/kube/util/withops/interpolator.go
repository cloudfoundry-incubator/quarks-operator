package withops

import (
	"github.com/cppforlife/go-patch/patch"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Interpolator renders BOSH manifests by operations files
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
		return errors.Wrapf(err, "Unmarshalling ops data %s failed", string(opsBytes))
	}

	ops, err := patch.NewOpsFromDefinitions(opDefs)
	if err != nil {
		return errors.Wrapf(err, "Building ops from opDefs failed")
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
		return []byte{}, errors.Wrapf(err, "Unmarshalling manifest obj %s failed in interpolator", string(manifestBytes))
	}

	// Apply ops
	if i.ops != nil {
		obj, err = i.ops.Apply(obj)
		if err != nil {
			return []byte{}, errors.Wrapf(err, "Applying ops on manifest obj failed in interpolator")
		}
	}

	// Interpolate from root
	obj, err = i.interpolateRoot(obj)
	if err != nil {
		return []byte{}, err
	}

	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "Marshalling manifest object in interpolator failed.")
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
	case map[string]interface{}:
		for k, v := range typedNode {
			evaluatedValue, err := i.interpolate(v)
			if err != nil {
				return nil, err
			}

			evaluatedKey, err := i.interpolate(k)
			if err != nil {
				return nil, err
			}

			stringKey, ok := evaluatedKey.(string)
			if !ok {
				return nil, errors.Errorf("interpolator only supports string keys, not '%T'", evaluatedKey)
			}

			delete(typedNode, k) // delete in case key has changed
			typedNode[stringKey] = evaluatedValue
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
