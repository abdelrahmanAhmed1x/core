// Package health separates cheap process liveness from dependency readiness.
package health

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"
	"time"
)

type Check func(context.Context) error
type Registry struct {
	mu      sync.RWMutex
	checks  map[string]Check
	timeout time.Duration
	slots   chan struct{}
}

func New(timeout time.Duration) *Registry {
	if timeout <= 0 {
		panic("health: timeout must be positive")
	}
	return &Registry{checks: make(map[string]Check), timeout: timeout, slots: make(chan struct{}, 32)}
}
func (r *Registry) Register(name string, check Check) {
	if name == "" || check == nil {
		panic("health: invalid check")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.checks[name]; ok {
		panic("health: duplicate check")
	}
	r.checks[name] = check
}

// Ready runs at most 32 dependency calls concurrently across requests. Check
// implementations must honor context. Stuck checks occupy bounded slots; later
// requests fail readiness instead of spawning unbounded replacement goroutines.
func (r *Registry) Ready(ctx context.Context) map[string]bool {
	r.mu.RLock()
	checks := make(map[string]Check, len(r.checks))
	for k, v := range r.checks {
		checks[k] = v
	}
	r.mu.RUnlock()
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	type result struct {
		name string
		ok   bool
	}
	ch := make(chan result, len(checks))
	out := make(map[string]bool, len(checks))
	for name, check := range checks {
		out[name] = false
		select {
		case r.slots <- struct{}{}:
			go func() { defer func() { <-r.slots }(); ch <- result{name, check(ctx) == nil} }()
		default:
			ch <- result{name, false}
		}
	}
	for range checks {
		select {
		case v := <-ch:
			out[v.name] = v.ok
		case <-ctx.Done():
			return out
		}
	}
	return out
}
func (r *Registry) Liveness(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// Readiness deliberately emits no check names or raw infrastructure errors.
func (r *Registry) Readiness(w http.ResponseWriter, req *http.Request) {
	for _, ok := range r.Ready(req.Context()) {
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}
	if req.Context().Err() != nil {
		w.WriteHeader(503)
		return
	}
	w.WriteHeader(200)
}
func (r *Registry) LiveHandler() gin.HandlerFunc  { return gin.WrapF(r.Liveness) }
func (r *Registry) ReadyHandler() gin.HandlerFunc { return gin.WrapF(r.Readiness) }
