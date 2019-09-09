package config

import (
	"time"

	"github.com/spf13/afero"
)

const (
	// CtxTimeOut is the default context.Context timeout
	CtxTimeOut = 30 * time.Second
	// MeltdownDuration is the duration of the meltdown period, in which we
	// postpone further reconciles for the same resource
	MeltdownDuration = 10 * time.Second
	// MeltdownRequeueAfter is the duration for which we delay the requeuing of the reconcile
	MeltdownRequeueAfter = 5 * time.Second
)

// Config controls the behaviour of different controllers
type Config struct {
	CtxTimeOut                    time.Duration
	MeltdownDuration              time.Duration
	MeltdownRequeueAfter          time.Duration
	Namespace                     string
	Provider                      string
	WebhookServerHost             string
	WebhookServerPort             int32
	Fs                            afero.Fs
	MaxBoshDeploymentWorkers      int
	MaxExtendedJobWorkers         int
	MaxExtendedSecretWorkers      int
	MaxExtendedStatefulSetWorkers int
	ApplyCRD                      bool
}

// NewConfig returns a new Config for a Manager of Controllers
func NewConfig(namespace string, provider string, host string, port int32, fs afero.Fs, maxBoshDeploymentWorkers, maxExtendedJobWorkers, maxExtendedSecretWorkers, maxExtendedStatefulSetWorkers int, applyCRD bool) *Config {
	return &Config{
		CtxTimeOut:                    CtxTimeOut,
		MeltdownDuration:              MeltdownDuration,
		MeltdownRequeueAfter:          MeltdownRequeueAfter,
		Namespace:                     namespace,
		Provider:                      provider,
		WebhookServerHost:             host,
		WebhookServerPort:             port,
		Fs:                            fs,
		MaxBoshDeploymentWorkers:      maxBoshDeploymentWorkers,
		MaxExtendedJobWorkers:         maxExtendedJobWorkers,
		MaxExtendedSecretWorkers:      maxExtendedSecretWorkers,
		MaxExtendedStatefulSetWorkers: maxExtendedStatefulSetWorkers,
		ApplyCRD:                      applyCRD,
	}
}
