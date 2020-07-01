package environment

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// QuarksCmds holds the quarks standalone components gexec binaries
type QuarksCmds struct {
	Job    QuarksJobCmd
	Secret QuarksSecretCmd
}

type quarksPath struct {
	QJob string
	QSec string
}

// NewQuarksCmds returns a new struct for the standalone commands
func NewQuarksCmds() *QuarksCmds {
	return &QuarksCmds{}
}

// Build builds all the standalone binaries
func (q *QuarksCmds) Build() error {
	err := q.Job.Build()
	if err != nil {
		return errors.Wrap(err, "failed to build quarks-job")
	}

	err = q.Secret.Build()
	if err != nil {
		return errors.Wrap(err, "failed to build quarks-secret")
	}
	return nil
}

// Marshal returns a JSON with the paths to the binaries
func (q *QuarksCmds) Marshal() []byte {
	bytes, _ := json.Marshal(quarksPath{QJob: q.Job.Path, QSec: q.Secret.Path})
	return bytes
}

// Unmarshal loads the binary paths from JSON
func (q *QuarksCmds) Unmarshal(data []byte) error {
	paths := &quarksPath{}
	err := json.Unmarshal(data, &paths)
	if err != nil {
		return err
	}
	q.Job.Path = string(paths.QJob)
	q.Secret.Path = string(paths.QSec)
	return nil
}
