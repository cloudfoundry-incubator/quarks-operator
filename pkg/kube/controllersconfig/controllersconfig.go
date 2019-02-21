package controllersconfig

import (
	"context"
	"time"
)

//ControllersConfig controls the behaviour of
//different controllers
type ControllersConfig struct {
	CtxTimeOut time.Duration
	CtxType    context.Context
	Namespace  string
}

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
func NewBackgroundContextWithTimeout(ctx context.Context, ctxTimeOut time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, ctxTimeOut)
}
