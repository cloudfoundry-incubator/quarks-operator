package config

import (
	"time"

	"github.com/spf13/afero"
)

const (
	// CtxTimeOut is the default context.Context timeout
	CtxTimeOut = 30 * time.Second
)

// Config controls the behaviour of different controllers
type Config struct {
	CtxTimeOut                    time.Duration
	Namespace                     string
	WebhookServerHost             string
	WebhookServerPort             int32
	Fs                            afero.Fs
	MaxBoshDeploymentWorkers      int
	MaxExtendedJobWorkers         int
	MaxExtendedSecretWorkers      int
	MaxExtendedStatefulSetWorkers int
}

// NewConfig returns a new Config for a Manager of Controllers
func NewConfig(namespace string, host string, port int32, fs afero.Fs, maxBoshDeploymentWorkers, maxExtendedJobWorkers, maxExtendedSecretWorkers, maxExtendedStatefulSetWorkers int) *Config {
	return &Config{
		CtxTimeOut:                    CtxTimeOut,
		Namespace:                     namespace,
		WebhookServerHost:             host,
		WebhookServerPort:             port,
		Fs:                            fs,
		MaxBoshDeploymentWorkers:      maxBoshDeploymentWorkers,
		MaxExtendedJobWorkers:         maxExtendedJobWorkers,
		MaxExtendedSecretWorkers:      maxExtendedSecretWorkers,
		MaxExtendedStatefulSetWorkers: maxExtendedStatefulSetWorkers,
	}
}
