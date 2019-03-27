package config

import (
	"time"

	"github.com/spf13/afero"
)

// Config controls the behaviour of different controllers
type Config struct {
	CtxTimeOut        time.Duration
	Namespace         string
	WebhookServerHost string
	WebhookServerPort int32
	Fs                afero.Fs
}
