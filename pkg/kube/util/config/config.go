package config

import (
	"time"

	"github.com/spf13/afero"
)

const (
	// CtxTimeOut is the default context.Context timeout
	CtxTimeOut = 10 * time.Second
)

// Config controls the behaviour of different controllers
type Config struct {
	CtxTimeOut        time.Duration
	Namespace         string
	WebhookServerHost string
	WebhookServerPort int32
	Fs                afero.Fs
}

func NewConfig(namespace string, host string, port int32, fs afero.Fs) *Config {
	return &Config{
		CtxTimeOut:        CtxTimeOut,
		Namespace:         namespace,
		WebhookServerHost: host,
		WebhookServerPort: port,
		Fs:                fs,
	}
}
