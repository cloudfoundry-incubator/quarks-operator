package util

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// NewTestLogger returns a ZAP logger for assertions, which also logs to
// /tmp/cf-operator-tests.log
func NewTestLogger() (obs *observer.ObservedLogs, log *zap.SugaredLogger) {
	// An in-memory zap core that can be used for assertions
	var memCore zapcore.Core
	memCore, obs = observer.New(zapcore.DebugLevel)

	// A zap core that writes to a temp file
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	f, err := os.Create("/tmp/cf-operator-tests.log")
	if err != nil {
		panic(fmt.Sprintf("can't create log file: %s\n", err.Error()))
	}
	fileCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.Lock(f),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return true
		}))

	log = zap.New(zapcore.NewTee(memCore, fileCore)).Sugar()
	return
}
