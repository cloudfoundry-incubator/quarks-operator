package fakes

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// FakeLogger wraps zapcores observer
type FakeLogger struct {
	Log         *zap.SugaredLogger
	LogRecorded *observer.ObservedLogs
}

// NewFakeLogger returns the wrapper and a zap sugared logger
func NewFakeLogger() (*FakeLogger, *zap.SugaredLogger) {
	core, logRecorder := observer.New(zapcore.DebugLevel)
	log := zap.New(core).Sugar()
	return &FakeLogger{
		Log:         log,
		LogRecorded: logRecorder,
	}, log
}

// AllMessages returns only the message part of existing logs to aid in assertions
func (fl *FakeLogger) AllMessages() (msgs []string) {
	for _, m := range fl.LogRecorded.All() {
		msgs = append(msgs, m.Message)
	}
	return
}

// PrintMessages prints all received messages
func (fl *FakeLogger) PrintMessages() {
	for _, m := range fl.LogRecorded.All() {
		fmt.Println(m.Message)
	}
}
