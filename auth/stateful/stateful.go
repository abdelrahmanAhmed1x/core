// Package stateful implements authoritative opaque sessions with pluggable storage.
package stateful

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var ErrInvalid = transport.ErrInvalid
var ErrCollision = errors.New("stateful: credential collision")

type Config struct {
	Namespace        string
	AbsoluteTTL      time.Duration
	IdleTTL          time.Duration
	OperationTimeout time.Duration
	Transport        transport.Config
}
type Manager[T any] struct {
	store SessionStore
	cfg   Config
}

// New preserves the Redis-backed constructor. For custom storage use NewWithStore.
func New[T any](client redis.UniversalClient, c Config) *Manager[T] {
	return NewWithStore[T](NewRedisStore(client), c)
}

// NewWithStore creates a manager using an authoritative session store.
// The caller owns the store and its lifecycle.
func NewWithStore[T any](store SessionStore, c Config) *Manager[T] {
	if store == nil || c.Namespace == "" || c.AbsoluteTTL < time.Millisecond || (c.IdleTTL > 0 && c.IdleTTL < time.Millisecond) || c.IdleTTL < 0 || c.OperationTimeout < 0 {
		panic("stateful: invalid configuration")
	}
	if c.OperationTimeout == 0 {
		c.OperationTimeout = 3 * time.Second
	}
	c.Transport = transport.Normalize(c.Transport)
	return &Manager[T]{store: store, cfg: c}
}

// Create linearizes with RevokeAll: creations before revoke are invalidated;
// creations after revoke remain valid.
func (m *Manager[T]) Create(ctx context.Context, userID string, p T) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("stateful: empty user ID")
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	for range 3 {
		raw, err := token.Random()
		if err != nil {
			return "", err
		}
		err = m.store.Create(ctx, CreateRequest{
			Namespace: m.cfg.Namespace, TokenHash: token.Hash(raw), UserHash: token.Hash(userID),
			Payload: payload, AbsoluteTTL: m.cfg.AbsoluteTTL, IdleTTL: m.cfg.IdleTTL,
		})
		if errors.Is(err, ErrCollision) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("stateful: create: %w", err)
		}
		return raw, nil
	}
	return "", ErrCollision
}

func (m *Manager[T]) read(ctx context.Context, s string, renew bool) (T, error) {
	var z T
	if !token.ValidOpaque(s) {
		return z, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	var payload []byte
	var err error
	if renew {
		payload, err = m.store.Renew(ctx, m.cfg.Namespace, token.Hash(s), m.cfg.IdleTTL)
	} else {
		payload, err = m.store.Verify(ctx, m.cfg.Namespace, token.Hash(s))
	}
	if err != nil {
		return z, fmt.Errorf("stateful: verify: %w", err)
	}
	if json.Unmarshal(payload, &z) != nil {
		var empty T
		return empty, ErrInvalid
	}
	return z, nil
}
func (m *Manager[T]) Verify(ctx context.Context, s string) (T, error) { return m.read(ctx, s, false) }

// Renew extends idle expiration only; absolute expiration never moves.
func (m *Manager[T]) Renew(ctx context.Context, s string) (T, error) { return m.read(ctx, s, true) }
func (m *Manager[T]) Revoke(ctx context.Context, s string) error {
	if !token.ValidOpaque(s) {
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	return m.store.Revoke(ctx, m.cfg.Namespace, token.Hash(s))
}
func (m *Manager[T]) RevokeAll(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	return m.store.RevokeAll(ctx, m.cfg.Namespace, token.Hash(userID), m.cfg.AbsoluteTTL)
}
func (m *Manager[T]) Middleware() gin.HandlerFunc {
	return transport.Middleware(m.cfg.Transport, false, m.Verify)
}
func (m *Manager[T]) OptionalMiddleware() gin.HandlerFunc {
	return transport.Middleware(m.cfg.Transport, true, m.Verify)
}
func (m *Manager[T]) Payload(c *gin.Context) (T, bool) { return transport.Payload[T](c) }
func (m *Manager[T]) Login(c *gin.Context, userID string, p T) error {
	s, e := m.Create(c.Request.Context(), userID, p)
	if e != nil {
		return e
	}
	transport.SetCookie(c.Writer, m.cfg.Transport, s, m.cfg.AbsoluteTTL)
	return nil
}
func (m *Manager[T]) Logout(c *gin.Context) error {
	s, e := transport.Extract(c.Request, m.cfg.Transport)
	if e != nil {
		return e
	}
	e = m.Revoke(c.Request.Context(), s)
	transport.SetCookie(c.Writer, m.cfg.Transport, "", 0)
	return e
}
