// Package ctxlog extends ctxlog with events
package ctxlog

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type event struct {
	object runtime.Object
	reason string
}

// EventLogger adds events and writes logs
type EventLogger interface {
	Infof(context.Context, string, ...interface{})
	Errorf(context.Context, string, ...interface{}) error
	Error(context.Context, ...interface{}) error
}

// WithEvent returns a struct to provide event enhanced logging methods
func WithEvent(object runtime.Object, reason string) EventLogger {
	return event{object: object, reason: reason}
}

// Infof logs and adds an info event
func (ev event) Infof(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Infof(format, v...)

	recorder := ExtractRecorder(ctx)
	recorder.Eventf(ev.object, corev1.EventTypeNormal, ev.reason, format, v...)
}

// Errorf uses the stored zap logger and the recorder to log an error, it returns an error like fmt.Errorf
// 'object' is the object this event is about. Event will make a reference-- or you may also
// pass a reference to the object directly.
// 'reason' is the reason this event is generated. 'reason' should be short and unique; it
// should be in UpperCamelCase format (starting with a capital letter). "reason" will be used
// to automate handling of events, so imagine people writing switch statements to handle them.
// You want to make that easy.
func (ev event) Errorf(ctx context.Context, format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)

	return ev.logAndError(ctx, msg)
}

// Error uses the stored zap logger and recorder
func (ev event) Error(ctx context.Context, parts ...interface{}) error {
	msg := fmt.Sprint(parts...)

	return ev.logAndError(ctx, msg)
}

func (ev event) logAndError(ctx context.Context, msg string) error {
	log := ExtractLogger(ctx)
	log.Error(msg)

	recorder := ExtractRecorder(ctx)
	recorder.Event(ev.object, corev1.EventTypeWarning, ev.reason, msg)

	// first letter of error should be lowercase, so wrap looks nice.
	// ASCII only
	if len(msg) > 0 {
		msg = strings.ToLower(string(msg[0])) + msg[1:]
	}
	return fmt.Errorf(msg)
}

// WarningEvent will create a warning event, without logging
func WarningEvent(ctx context.Context, object runtime.Object, reason, msg string) {
	recorder := ExtractRecorder(ctx)
	recorder.Event(object, corev1.EventTypeWarning, reason, msg)
}
