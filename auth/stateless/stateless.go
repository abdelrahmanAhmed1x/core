// Package stateless implements independently verifiable access and refresh JWTs.
// Refreshing does not invalidate earlier tokens. There is no server-side logout.
package stateless

import (
	"context"
	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/gin-gonic/gin"
	"time"
)

type Config struct {
	Issuer          string
	AccessAudience  string
	RefreshAudience string
	AccessKey       string
	RefreshKey      string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	Leeway          time.Duration
	Transport       transport.Config
}
type Pair struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
}
type Manager[T any] struct{ access, refresh token.Config }

func New[T any](c Config) *Manager[T] {
	if c.AccessKey == c.RefreshKey || c.AccessAudience == c.RefreshAudience {
		panic("stateless: distinct keys and audiences required")
	}
	a := token.Normalize(token.Config{SecretKey: c.AccessKey, Issuer: c.Issuer, Audience: c.AccessAudience, TTL: c.AccessTTL, Leeway: c.Leeway, Transport: c.Transport})
	r := token.Normalize(token.Config{SecretKey: c.RefreshKey, Issuer: c.Issuer, Audience: c.RefreshAudience, TTL: c.RefreshTTL, Leeway: c.Leeway, Transport: c.Transport})
	return &Manager[T]{a, r}
}
func (m *Manager[T]) Issue(p T) (Pair, error) {
	a, e := token.Issue(m.access, p, "access")
	if e != nil {
		return Pair{}, e
	}
	r, e := token.Issue(m.refresh, p, "refresh")
	return Pair{a, r}, e
}
func (m *Manager[T]) VerifyAccess(s string) (T, error) { return token.Verify[T](m.access, s, "access") }
func (m *Manager[T]) VerifyRefresh(s string) (T, error) {
	return token.Verify[T](m.refresh, s, "refresh")
}
func (m *Manager[T]) Refresh(s string) (Pair, error) {
	p, e := m.VerifyRefresh(s)
	if e != nil {
		return Pair{}, e
	}
	return m.Issue(p)
}
func (m *Manager[T]) Middleware() gin.HandlerFunc         { return m.middleware(false) }
func (m *Manager[T]) OptionalMiddleware() gin.HandlerFunc { return m.middleware(true) }
func (m *Manager[T]) middleware(optional bool) gin.HandlerFunc {
	return transport.Middleware(m.access.Transport, optional, func(_ context.Context, s string) (T, error) { return m.VerifyAccess(s) })
}
func (m *Manager[T]) Payload(c *gin.Context) (T, bool) { return transport.Payload[T](c) }
