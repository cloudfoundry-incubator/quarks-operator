package withops

import (
	"github.com/SUSE/go-patch/patch"
	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Interpolator renders BOSH manifests by operations files
type Interpolator interface {
	AddOps(opsBytes []byte) error
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

// AddOps unmarshals ops definitions, processes them and holds them in memory
func (i *InterpolatorImpl) AddOps(opsBytes []byte) error {
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
	tpl := boshtpl.NewTemplate(manifestBytes)

	// Following options are empty for quarks-operator
	evalOpts := boshtpl.EvaluateOpts{
		ExpectAllKeys:     false,
		ExpectAllVarsUsed: false,
	}

	bytes, err := tpl.Evaluate(boshtpl.StaticVariables{}, i.ops, evalOpts)
	if err != nil {
		return nil, errors.Wrapf(err, "could not evaluate variables")
	}
	return bytes, nil
}
