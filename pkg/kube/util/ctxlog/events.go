// Package ctxlog extends ctxlog with events
package ctxlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Event holds information about a k8s events we create via
// controller-runtime's event recorder
type Event struct {
	object runtime.Object
	reason string
}

// PredicateEvent is used to debug controller predicates
type PredicateEvent struct {
	Event
}

// MappingEvent is used to debug EnqueueRequestsFromMapFunc in controllers
type MappingEvent struct {
	Event
}

const (
	// ReasonPredicates is used for controller predicate related logging
	ReasonPredicates = "Predicates"
	// ReasonMapping is used for controller EnqueueRequestsFromMapFunc related logging
	ReasonMapping = "Mapping"
)

// ReconcileEventsFromSource for defining useful logs when defining
// a mapping between a watched object and a
// reconcile one
type ReconcileEventsFromSource struct {
	ReconciliationObjectName string `json:"reconciliationObjectName"`
	ReconciliationObjectKind string `json:"reconciliationObjectKind"`
	PredicateObjectName      string `json:"predicateObjectName"`
	PredicateObjectKind      string `json:"predicateObjectKind"`
	Namespace                string `json:"namespace"`
	Message                  string `json:"message"`
	Type                     string `json:"type"`
}

// WithEvent returns a struct to provide event enhanced logging methods
// 'object' is the object this event is about. Event will make a reference-- or you may also
// pass a reference to the object directly.
// 'reason' is the reason this event is generated. 'reason' should be short and unique; it
// should be in UpperCamelCase format (starting with a capital letter). "reason" will be used
// to automate handling of events, so imagine people writing switch statements to handle them.
// You want to make that easy.
func WithEvent(object runtime.Object, reason string) Event {
	return Event{object: object, reason: reason}
}

// NewPredicateEvent returns a log event with the 'predicate' reason
func NewPredicateEvent(object runtime.Object) PredicateEvent {
	return PredicateEvent{Event{object: object, reason: ReasonPredicates}}
}

// NewMappingEvent returns a log event with the 'mapping' reason
func NewMappingEvent(object runtime.Object) MappingEvent {
	return MappingEvent{Event{object: object, reason: ReasonMapping}}
}

// Debug is used for predicate logging in the controllers
func (ev PredicateEvent) Debug(ctx context.Context, meta metav1.Object, resource string, msg string) {
	ev.debugJSON(
		ctx,
		ReconcileEventsFromSource{
			ReconciliationObjectName: meta.GetName(),
			ReconciliationObjectKind: resource,
			PredicateObjectName:      meta.GetName(),
			PredicateObjectKind:      resource,
			Namespace:                meta.GetNamespace(),
			Type:                     ev.reason,
			Message:                  msg,
		},
	)
}

// Debug is used for logging in EnqueueRequestsFromMapFunc
func (ev MappingEvent) Debug(ctx context.Context, reconciliation reconcile.Request, crd string, objName string, objType string) {
	ev.debugJSON(
		ctx,
		ReconcileEventsFromSource{
			ReconciliationObjectName: reconciliation.Name,
			ReconciliationObjectKind: crd,
			PredicateObjectName:      objName,
			PredicateObjectKind:      objType,
			Namespace:                reconciliation.Namespace,
			Type:                     ev.reason,
			Message:                  fmt.Sprintf("Enqueuing reconcile requests: fan-out updates from %s, type %s into %s", objName, objType, reconciliation.Name),
		},
	)
}

// debugJSON logs a message and adds an info event in json format
func (ev Event) debugJSON(ctx context.Context, objectInfo interface{}) {
	jsonData, _ := json.Marshal(objectInfo)
	recorder := ExtractRecorder(ctx)
	recorder.Event(ev.object, corev1.EventTypeNormal, ev.reason, string(jsonData))

	// treat JSON data as a string map and extract message
	var result map[string]string
	json.Unmarshal([]byte(jsonData), &result)
	if msg, ok := result["message"]; ok {
		log := ExtractLoggerWithOptions(ctx, zap.AddCallerSkip(1))
		log.Debug(msg)
	}
}

// Debugf logs and adds an info event
func (ev Event) Debugf(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLoggerWithOptions(ctx, zap.AddCallerSkip(1))
	log.Debugf(format, v...)

	recorder := ExtractRecorder(ctx)
	recorder.Eventf(ev.object, corev1.EventTypeNormal, ev.reason, format, v...)
}

// Infof logs and adds an info event
func (ev Event) Infof(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLoggerWithOptions(ctx, zap.AddCallerSkip(1))
	log.Infof(format, v...)

	recorder := ExtractRecorder(ctx)
	recorder.Eventf(ev.object, corev1.EventTypeNormal, ev.reason, format, v...)
}

// Errorf uses the stored zap logger and the recorder to log an error, it returns an error like fmt.Errorf
func (ev Event) Errorf(ctx context.Context, format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)

	return ev.logAndError(ctx, msg)
}

// Error uses the stored zap logger and recorder
func (ev Event) Error(ctx context.Context, parts ...interface{}) error {
	msg := fmt.Sprint(parts...)

	return ev.logAndError(ctx, msg)
}

func (ev Event) logAndError(ctx context.Context, msg string) error {
	log := ExtractLoggerWithOptions(ctx, zap.AddCallerSkip(1))
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
