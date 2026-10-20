// Package tiered composes L1/L2 caches with bounded staleness, not coherence.
// Other processes can retain L1 copies for MaxL1TTL after a write/delete.
package tiered

import (
	"context"
	"errors"
	"github.com/abdelrahmanAhmed1x/core/cache"
	"time"
)

type Cache[T any] struct {
	l1, l2   cache.Cache[T]
	ttl      time.Duration
	observer cache.Observer
}

func New[T any](l1, l2 cache.Cache[T], maxL1TTL time.Duration, observer cache.Observer) *Cache[T] {
	if l1 == nil || l2 == nil || maxL1TTL <= 0 {
		panic("tiered: invalid configuration")
	}
	return &Cache[T]{l1, l2, maxL1TTL, observer}
}
func (m *Cache[T]) Get(ctx context.Context, k string) (cache.Entry[T], error) {
	v, e := m.l1.Get(ctx, k)
	if e == nil {
		return v, nil
	}
	if !errors.Is(e, cache.ErrMiss) {
		cache.Observe(m.observer, "l1.error", time.Now(), e)
	}
	v, e = m.l2.Get(ctx, k)
	if e != nil {
		return v, e
	}
	ttl := min(m.ttl, time.Until(v.ExpiresAt))
	if ttl > 0 {
		e = m.l1.Set(ctx, k, v.Value, ttl)
		cache.Observe(m.observer, "l1.populate", time.Now(), e)
	}
	return v, nil
}
func (m *Cache[T]) Set(ctx context.Context, k string, v T, ttl time.Duration) error {
	if ttl <= 0 {
		return cache.ErrTTL
	}
	if e := m.l2.Set(ctx, k, v, ttl); e != nil {
		return e
	}
	return m.l1.Set(ctx, k, v, min(ttl, m.ttl))
}
func (m *Cache[T]) Delete(ctx context.Context, k string) error {
	return errors.Join(m.l1.Delete(ctx, k), m.l2.Delete(ctx, k))
}
