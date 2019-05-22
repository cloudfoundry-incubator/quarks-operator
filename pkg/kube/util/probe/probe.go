package probe

import (
	"context"
	"net/http"
	"sync"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

const (
	// HTTPReadyzEndpoint route
	HTTPReadyzEndpoint = "/readyz"
)

// ReadyzProbe to safely set
// the readinessProbe endpoint
type ReadyzProbe struct {
	Ready bool
	mux   sync.Mutex
	Ctx   context.Context
}

// SetReady to set to true the Ready field
func (r *ReadyzProbe) SetReady() {
	r.mux.Lock()
	r.Ready = true
	r.mux.Unlock()
	ctxlog.Info(r.Ctx, "The /readyz route is ready to listen and serve")
}

// GetRoute gives back the https endpoint
func (r *ReadyzProbe) GetRoute() string {
	ReadyzRoute := HTTPReadyzEndpoint
	return ReadyzRoute
}

// ReadyzHandler writes back a 200 http code
// only when the operator manager was set and
// started, otherwise it will return a 500 http code.
func (r *ReadyzProbe) ReadyzHandler(w http.ResponseWriter, _ *http.Request) {
	r.mux.Lock()
	isReady := r.Ready
	r.mux.Unlock()
	if isReady {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
