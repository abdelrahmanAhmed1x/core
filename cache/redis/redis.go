// Package redis provides a Redis/Dragonfly cache and never owns the client.
package redis

import (
	"context"
	"errors"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/cache"
	r "github.com/redis/go-redis/v9"
	"time"
)

type Config struct {
	Namespace string
	Timeout   time.Duration
	MaxBytes  int
	Observer  cache.Observer
}
type Cache[T any] struct {
	client r.UniversalClient
	cfg    Config
	codec  cache.Codec[T]
}

func New[T any](client r.UniversalClient, c Config, codec cache.Codec[T]) *Cache[T] {
	if client == nil || c.Namespace == "" || c.Timeout < 0 || c.MaxBytes < 0 {
		panic("redis cache: invalid configuration")
	}
	if c.Timeout == 0 {
		c.Timeout = 2 * time.Second
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = 1 << 20
	}
	if codec == nil {
		codec = cache.JSON[T]{}
	}
	return &Cache[T]{client, c, codec}
}

var get = r.NewScript(`local v=redis.call('GET',KEYS[1]);if not v then return nil end;return {v,redis.call('PTTL',KEYS[1])}`)

func (m *Cache[T]) Get(ctx context.Context, k string) (out cache.Entry[T], err error) {
	start := time.Now()
	defer func() { cache.Observe(m.cfg.Observer, "l2.get", start, err) }()
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	result, err := get.Run(ctx, m.client, []string{m.cfg.Namespace + ":" + k}).Slice()
	if errors.Is(err, r.Nil) {
		return out, cache.ErrMiss
	}
	if err != nil {
		return out, err
	}
	if len(result) != 2 {
		return out, fmt.Errorf("cache: invalid Redis reply")
	}
	b, ok := result[0].(string)
	if !ok {
		return out, fmt.Errorf("cache: invalid Redis value")
	}
	ttl, ok := result[1].(int64)
	if !ok || ttl <= 0 {
		return out, cache.ErrMiss
	}
	if len(b) > m.cfg.MaxBytes {
		return out, cache.ErrTooLarge
	}
	out.Value, err = m.codec.Decode([]byte(b))
	if err != nil {
		err = fmt.Errorf("%w: %w", cache.ErrCodec, err)
		cache.Observe(m.cfg.Observer, "codec.decode", start, err)
	}
	out.ExpiresAt = start.Add(time.Duration(ttl) * time.Millisecond)
	if !time.Now().Before(out.ExpiresAt) {
		return cache.Entry[T]{}, cache.ErrMiss
	}
	return out, err
}
func (m *Cache[T]) Set(ctx context.Context, k string, v T, ttl time.Duration) (err error) {
	start := time.Now()
	defer func() { cache.Observe(m.cfg.Observer, "l2.set", start, err) }()
	if ttl < time.Millisecond {
		return cache.ErrTTL
	}
	if err = ctx.Err(); err != nil {
		return
	}
	b, err := m.codec.Encode(v)
	if err != nil {
		err = fmt.Errorf("%w: %w", cache.ErrCodec, err)
		cache.Observe(m.cfg.Observer, "codec.encode", start, err)
		return
	}
	if len(b) > m.cfg.MaxBytes {
		return cache.ErrTooLarge
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	return m.client.Set(ctx, m.cfg.Namespace+":"+k, b, ttl).Err()
}
func (m *Cache[T]) Delete(ctx context.Context, k string) error {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	return m.client.Del(ctx, m.cfg.Namespace+":"+k).Err()
}
