package testhelper

import (
	"fmt"
	"os"
	"path"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// NewTestLogger returns a ZAP logger for assertions, which also logs to
// <tmpdir>/cf-operator-tests.log
func NewTestLogger() (obs *observer.ObservedLogs, log *zap.SugaredLogger) {
	return NewTestLoggerWithPath(LogfilePath("cf-operator-tests.log"))
}

// NewTestLoggerWithPath returns a logger which logs to path
func NewTestLoggerWithPath(path string) (obs *observer.ObservedLogs, log *zap.SugaredLogger) {
	// An in-memory zap core that can be used for assertions
	var memCore zapcore.Core
	memCore, obs = observer.New(zapcore.DebugLevel)

	// A zap core that writes to a temp file
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	f, err := os.Create(path)
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

// LogfilePath returns the path for the given filename in the testing tmp directory
func LogfilePath(filename string) string {
	tmpDir := os.Getenv("CF_OPERATOR_TESTING_TMP")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	return path.Join(tmpDir, filename)
}
