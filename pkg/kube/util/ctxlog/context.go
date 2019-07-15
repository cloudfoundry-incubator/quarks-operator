package ctxlog

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/record"
)

type ctxLogger struct{}
type ctxRecorder struct{}

// key must be comparable and should not be of type string
var (
	ctxLoggerKey   = &ctxLogger{}
	ctxRecorderKey = &ctxRecorder{}
	nopLogger      = zap.NewNop().Sugar()
	nopRecorder    = record.FakeRecorder{}
)

// NewParentContext returns a new context with a logger
func NewParentContext(log *zap.SugaredLogger) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxLoggerKey, log)
	return ctx
}

// NewContextWithRecorder returns a new child context with the named
// recorder and log inside
func NewContextWithRecorder(ctx context.Context, name string, recorder record.EventRecorder) context.Context {
	ctx = context.WithValue(ctx, ctxRecorderKey, recorder)
	log := ExtractLogger(ctx)
	log = log.Named(name)
	return context.WithValue(ctx, ctxLoggerKey, log)
}

// ExtractLoggerWithOptions returns a logger with different options than the parent
func ExtractLoggerWithOptions(ctx context.Context, options ...zap.Option) *zap.SugaredLogger {
	log := ExtractLogger(ctx)
	return log.Desugar().WithOptions(options...).Sugar()
}

// ExtractLogger returns the logger from the context
func ExtractLogger(ctx context.Context) *zap.SugaredLogger {
	log, ok := ctx.Value(ctxLoggerKey).(*zap.SugaredLogger)
	if !ok || log == nil {
		return nopLogger
	}
	return log
}

// ExtractRecorder returns the event recorder from the context
func ExtractRecorder(ctx context.Context) record.EventRecorder {
	recorder, ok := ctx.Value(ctxRecorderKey).(record.EventRecorder)
	if !ok || recorder == nil {
		return &nopRecorder
	}
	return recorder
}
