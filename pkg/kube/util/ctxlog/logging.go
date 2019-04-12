package ctxlog

import "context"

// Debug uses the stored zap logger
func Debug(ctx context.Context, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Debug(v...)
}

// Info uses the stored zap logger
func Info(ctx context.Context, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Info(v...)
}

// Error uses the stored zap logger
func Error(ctx context.Context, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Error(v...)
}

// Debugf uses the stored zap logger
func Debugf(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Debugf(format, v...)
}

// Infof uses the stored zap logger
func Infof(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Infof(format, v...)
}

// Errorf uses the stored zap logger
func Errorf(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Errorf(format, v...)
}
