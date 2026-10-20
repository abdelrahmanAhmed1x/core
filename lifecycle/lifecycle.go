// Package lifecycle coordinates explicitly registered, reverse-order cleanup.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type cleanup struct {
	name string
	fn   func(context.Context) error
}
type Manager struct {
	mu      sync.Mutex
	timeout time.Duration
	hooks   []cleanup
	started bool
	done    chan struct{}
	err     error
}

func New(timeout time.Duration) *Manager {
	if timeout <= 0 {
		panic("lifecycle: timeout must be positive")
	}
	return &Manager{timeout: timeout, done: make(chan struct{})}
}
func (m *Manager) Register(name string, fn func(context.Context) error) error {
	if name == "" || fn == nil {
		return errors.New("lifecycle: invalid hook")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("lifecycle: shutdown started")
	}
	m.hooks = append(m.hooks, cleanup{name, fn})
	return nil
}
func (m *Manager) RegisterHTTP(name string, server *http.Server) error {
	if server == nil {
		return errors.New("lifecycle: nil server")
	}
	return m.Register(name, server.Shutdown)
}

// Shutdown is idempotent; hooks run once. The timeout is cooperative: hooks must
// honor ctx. Hooks are not abandoned in background goroutines. Concurrent callers
// can cancel their own wait independently.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		select {
		case <-m.done:
			return m.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.started = true
	hooks := m.hooks
	m.mu.Unlock()
	work, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		if e := call(work, hooks[i]); e != nil {
			errs = append(errs, fmt.Errorf("%s: %w", hooks[i].name, e))
		}
	}
	if e := work.Err(); e != nil {
		errs = append(errs, e)
	}
	m.mu.Lock()
	m.err = errors.Join(errs...)
	close(m.done)
	m.mu.Unlock()
	return m.err
}
func call(ctx context.Context, c cleanup) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("cleanup panicked")
		}
	}()
	return c.fn(ctx)
}
