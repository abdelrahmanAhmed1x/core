// Package stateful implements authoritative Redis-backed opaque sessions.
package stateful

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"time"
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
	client redis.UniversalClient
	cfg    Config
	prefix string
}
type record struct {
	Payload string `json:"payload"`
	User    string `json:"user"`
	Epoch   string `json:"epoch"`
	Expires int64  `json:"expires"`
	Idle    int64  `json:"idle"`
}

func New[T any](client redis.UniversalClient, c Config) *Manager[T] {
	if client == nil || c.Namespace == "" || c.AbsoluteTTL < time.Millisecond || (c.IdleTTL > 0 && c.IdleTTL < time.Millisecond) || c.IdleTTL < 0 || c.OperationTimeout < 0 {
		panic("stateful: invalid configuration")
	}
	if c.OperationTimeout == 0 {
		c.OperationTimeout = 3 * time.Second
	}
	c.Transport = transport.Normalize(c.Transport)
	return &Manager[T]{client, c, "core:session:{" + token.Hash(c.Namespace) + "}:"}
}

var create = redis.NewScript(`
if redis.call('EXISTS',KEYS[1])==1 then return 0 end
local epoch=redis.call('GET',KEYS[2]) or ARGV[2]
local t=redis.call('TIME'); local now=t[1]*1000+math.floor(t[2]/1000)
local data=cjson.decode(ARGV[1]); data.epoch=epoch;data.expires=now+tonumber(ARGV[3]);data.idle=0
local ttl=tonumber(ARGV[3]);if tonumber(ARGV[4])>0 then ttl=math.min(ttl,tonumber(ARGV[4]));data.idle=now+ttl end
if redis.call('PTTL',KEYS[2])<tonumber(ARGV[3]) then redis.call('SET',KEYS[2],epoch,'PX',ARGV[3]) end
redis.call('SET',KEYS[1],cjson.encode(data),'PX',ttl);return 1`)

// Create linearizes with RevokeAll: creations before revoke are invalidated;
// creations after revoke use the new generation and remain valid.
func (m *Manager[T]) Create(ctx context.Context, userID string, p T) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("stateful: empty user ID")
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	payload, e := json.Marshal(p)
	if e != nil {
		return "", e
	}
	data, e := json.Marshal(record{Payload: string(payload), User: token.Hash(userID)})
	if e != nil {
		return "", e
	}
	for range 3 {
		raw, e := token.Random()
		if e != nil {
			return "", e
		}
		epoch, e := token.Random()
		if e != nil {
			return "", e
		}
		n, e := create.Run(ctx, m.client, []string{m.key(raw), m.prefix + "user:" + token.Hash(userID)}, data, epoch, m.cfg.AbsoluteTTL.Milliseconds(), m.cfg.IdleTTL.Milliseconds()).Int()
		if e != nil {
			return "", fmt.Errorf("stateful: create: %w", e)
		}
		if n == 1 {
			return raw, nil
		}
	}
	return "", ErrCollision
}
func (m *Manager[T]) key(s string) string { return m.prefix + "token:" + token.Hash(s) }

var verify = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return '' end
local ok,data=pcall(cjson.decode,raw);if not ok then return '' end
if data.user~=ARGV[1] or data.epoch~=redis.call('GET',KEYS[2]) then return '' end
local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000)
if type(data.expires)~='number' or type(data.idle)~='number' or data.expires<=now or (data.idle>0 and data.idle<=now) then return '' end
if tonumber(ARGV[2])>0 then
 local ttl=math.min(data.expires-now,tonumber(ARGV[2]));data.idle=now+ttl
 redis.call('SET',KEYS[1],cjson.encode(data),'PX',ttl)
end
return raw`)

func (m *Manager[T]) read(ctx context.Context, s string, renew bool) (T, error) {
	var z T
	if !token.ValidOpaque(s) {
		return z, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	raw, e := m.client.Get(ctx, m.key(s)).Bytes()
	if errors.Is(e, redis.Nil) {
		return z, ErrInvalid
	}
	if e != nil {
		return z, fmt.Errorf("stateful: read: %w", e)
	}
	var r record
	if json.Unmarshal(raw, &r) != nil || len(r.User) != 64 || r.Epoch == "" || r.Expires <= 0 {
		return z, ErrInvalid
	}
	idle := int64(0)
	if renew {
		idle = m.cfg.IdleTTL.Milliseconds()
	}
	result, err := verify.Run(ctx, m.client, []string{m.key(s), m.prefix + "user:" + r.User}, r.User, idle).Text()
	raw = []byte(result)
	e = err
	if e != nil {
		return z, fmt.Errorf("stateful: verify: %w", e)
	}
	if len(raw) == 0 || json.Unmarshal(raw, &r) != nil {
		return z, ErrInvalid
	}
	if json.Unmarshal([]byte(r.Payload), &z) != nil {
		return z, ErrInvalid
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
	return m.client.Del(ctx, m.key(s)).Err()
}
func (m *Manager[T]) RevokeAll(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalid
	}
	epoch, e := token.Random()
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()
	return m.client.Set(ctx, m.prefix+"user:"+token.Hash(userID), epoch, m.cfg.AbsoluteTTL).Err()
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
