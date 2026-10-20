// Package single implements one short-lived JWT without server-side revocation.
package single

import (
	"context"
	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/gin-gonic/gin"
	"time"
)

// Config requires a high-entropy HS256 key, issuer, audience and finite TTL.
// Transport defaults to a Secure __Host-session cookie plus bearer extraction.
type Config struct {
	SecretKey string
	Issuer    string
	Audience  string
	TTL       time.Duration
	Leeway    time.Duration
	Transport transport.Config
}

// Manager issues and verifies one credential type. It has no server-side state.
type Manager[T any] struct{ cfg token.Config }

// New panics on unsafe configuration. The algorithm is fixed to HS256.
func New[T any](cfg Config) *Manager[T] { return &Manager[T]{token.Normalize(token.Config(cfg))} }

// Issue signs a new access token after the application has authenticated p.
func (m *Manager[T]) Issue(p T) (string, error) { return token.Issue(m.cfg, p, "access") }

// Verify returns ErrInvalid for any invalid credential without exposing claims.
func (m *Manager[T]) Verify(s string) (T, error)          { return token.Verify[T](m.cfg, s, "access") }
func (m *Manager[T]) Middleware() gin.HandlerFunc         { return m.middleware(false) }
func (m *Manager[T]) OptionalMiddleware() gin.HandlerFunc { return m.middleware(true) }
func (m *Manager[T]) middleware(optional bool) gin.HandlerFunc {
	return transport.Middleware(m.cfg.Transport, optional, func(_ context.Context, s string) (T, error) { return m.Verify(s) })
}
func (m *Manager[T]) Payload(c *gin.Context) (T, bool) { return transport.Payload[T](c) }
