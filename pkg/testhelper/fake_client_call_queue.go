package testhelper

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
)

type expectFunc func(context.Context, runtime.Object) error

// CallQueue represents a list of expected function calls
type CallQueue struct {
	n     int
	calls []expectFunc
}

// NewCallQueue returns a new list of expected functions
func NewCallQueue(funcs ...expectFunc) CallQueue {
	return CallQueue{calls: funcs}
}

// Calls can be used with counterfeiters *Calls functions
// to set the stub functions
func (q *CallQueue) Calls(context context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
	n := q.n
	if n >= len(q.calls) {
		return nil
	}
	err := q.calls[n](context, object)
	q.n = n + 1
	return err
}
