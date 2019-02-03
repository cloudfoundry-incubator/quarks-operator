package clientcontext

import (
	"context"
	"time"
)

const (
	timeOutInMiliseconds int = 500
)

//NewContext returns a non-nil empty context, for usage
//when it is unclear which context to use.
func NewContext() context.Context {
	return context.TODO()
}

//NewBackgroundContext returns a top level context that
//has no values and deadline
func NewBackgroundContext() context.Context {
	return context.Background()
}

//WithValue returns a copy of parent where the value associated
//with the key is val
func WithValue(parent context.Context, key interface{}, val interface{}) context.Context {
	return context.WithValue(parent, key, val)
}

//NewBackgroundContextWithTimeout returns a context that if
//cancelled it will release resources associated with it.
func NewBackgroundContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(NewBackgroundContext(), time.Duration(timeOutInMiliseconds)*time.Millisecond)
}
