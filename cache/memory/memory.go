// Package memory provides synchronous LRU admission with lazy TTL expiration.
// Capacity is bounded by Entries * (MaxBytes plus bounded key/entry overhead).
// There are no workers and no shutdown requirement.
package memory

import (
	"context"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/cache"
	lru "github.com/hashicorp/golang-lru/v2"
	"time"
)

type Config struct {
	Entries   int
	MaxBytes  int
	Namespace string
	Observer  cache.Observer
}
type stored struct {
	data    []byte
	expires time.Time
}
type Cache[T any] struct {
	lru   *lru.Cache[string, stored]
	codec cache.Codec[T]
	cfg   Config
}

func New[T any](c Config, codec cache.Codec[T]) *Cache[T] {
	if c.Entries <= 0 || c.MaxBytes < 0 {
		panic("memory: invalid capacity")
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = 1 << 20
	}
	if codec == nil {
		codec = cache.JSON[T]{}
	}
	l, e := lru.New[string, stored](c.Entries)
	if e != nil {
		panic(e)
	}
	return &Cache[T]{l, codec, c}
}
func (m *Cache[T]) Get(ctx context.Context, k string) (out cache.Entry[T], err error) {
	start := time.Now()
	defer func() { cache.Observe(m.cfg.Observer, "l1.get", start, err) }()
	if err = ctx.Err(); err != nil {
		return
	}
	v, ok := m.lru.Get(m.cfg.Namespace + k)
	if !ok || !time.Now().Before(v.expires) {
		err = cache.ErrMiss
		return
	}
	out.Value, err = m.codec.Decode(v.data)
	if err != nil {
		err = fmt.Errorf("%w: %w", cache.ErrCodec, err)
		cache.Observe(m.cfg.Observer, "codec.decode", start, err)
	}
	out.ExpiresAt = v.expires
	return
}
func (m *Cache[T]) Set(ctx context.Context, k string, v T, ttl time.Duration) (err error) {
	start := time.Now()
	defer func() { cache.Observe(m.cfg.Observer, "l1.set", start, err) }()
	if err = ctx.Err(); err != nil {
		return
	}
	if ttl <= 0 {
		return cache.ErrTTL
	}
	if len(k)+len(m.cfg.Namespace) > 4096 {
		return fmt.Errorf("memory: key too long")
	}
	b, err := m.codec.Encode(v)
	if err != nil {
		err = fmt.Errorf("%w: %w", cache.ErrCodec, err)
		cache.Observe(m.cfg.Observer, "codec.encode", start, err)
		return err
	}
	if len(b) > m.cfg.MaxBytes {
		return cache.ErrTooLarge
	}
	evicted := m.lru.Add(m.cfg.Namespace+k, stored{b, time.Now().Add(ttl)})
	if evicted {
		cache.Observe(m.cfg.Observer, "l1.eviction", start, nil)
	}
	return nil
}
func (m *Cache[T]) Delete(ctx context.Context, k string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	m.lru.Remove(m.cfg.Namespace + k)
	return nil
}
func (m *Cache[T]) Purge() { m.lru.Purge() }
