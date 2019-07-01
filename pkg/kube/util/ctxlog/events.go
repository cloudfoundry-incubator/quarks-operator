// Package ctxlog extends ctxlog with events
package ctxlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type event struct {
	object runtime.Object
	reason string
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

// EventLogger adds events and writes logs
type EventLogger interface {
	Infof(context.Context, string, ...interface{})
	Debugf(context.Context, string, ...interface{})
	DebugPredicate(context.Context, metav1.Object, string, string)
	DebugMapping(context.Context, reconcile.Request, string, string, string)
	DebugJSON(context.Context, interface{})
	Errorf(context.Context, string, ...interface{}) error
	Error(context.Context, ...interface{}) error
}

var _ EventLogger = event{}

// WithEvent returns a struct to provide event enhanced logging methods
func WithEvent(object runtime.Object, reason string) EventLogger {
	return event{object: object, reason: reason}
}

// WithPredicateEvent returns a log event with the 'predicate' reason
func WithPredicateEvent(object runtime.Object) EventLogger {
	return event{object: object, reason: ReasonPredicates}
}

// WithMappingEvent returns a log event with the 'mapping' reason
func WithMappingEvent(object runtime.Object) EventLogger {
	return event{object: object, reason: ReasonMapping}
}

// DebugPredicate is used for predicate logging in the controllers
func (ev event) DebugPredicate(ctx context.Context, meta metav1.Object, resource string, msg string) {
	ev.DebugJSON(
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

// DebugPredicate is used for logging in EnqueueRequestsFromMapFunc
func (ev event) DebugMapping(ctx context.Context, reconciliation reconcile.Request, crd string, objName string, objType string) {
	ev.DebugJSON(
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

// DebugJSON logs a message and adds an info event in json format
func (ev event) DebugJSON(ctx context.Context, objectInfo interface{}) {
	jsonData, _ := json.Marshal(objectInfo)
	recorder := ExtractRecorder(ctx)
	recorder.Event(ev.object, corev1.EventTypeNormal, ev.reason, string(jsonData))

	// treat JSON data as a string map and extract message
	var result map[string]string
	json.Unmarshal([]byte(jsonData), &result)
	if msg, ok := result["message"]; ok {
		log := ExtractLogger(ctx)
		log.Debug(msg)
	}
}

// Debugf logs and adds an info event
func (ev event) Debugf(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Debugf(format, v...)

	recorder := ExtractRecorder(ctx)
	recorder.Eventf(ev.object, corev1.EventTypeNormal, ev.reason, format, v...)
}

// GenReconcilePredicatesObject ...
func GenReconcilePredicatesObject(ns string, t string, msg string, rKind string, pKind string) ReconcileEventsFromSource {
	return ReconcileEventsFromSource{}
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
