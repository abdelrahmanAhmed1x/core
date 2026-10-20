// Package cache defines typed, expiring application-data caches. It must not be
// used as a stale local authority for session revocation or authorization.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrLoaderPanic = errors.New("cache: source loader panicked")
	ErrCodec       = errors.New("cache: codec failure")
	ErrMiss        = errors.New("cache: miss")
	ErrTTL         = errors.New("cache: TTL must be positive")
	ErrTooLarge    = errors.New("cache: payload exceeds limit")
	ErrNotFound    = errors.New("cache: source not found")
	ErrBusy        = errors.New("cache: loader concurrency exhausted")
)

type Entry[T any] struct {
	Value     T
	ExpiresAt time.Time
}
type Cache[T any] interface {
	Get(context.Context, string) (Entry[T], error)
	Set(context.Context, string, T, time.Duration) error
	Delete(context.Context, string) error
}
type Codec[T any] interface {
	Encode(T) ([]byte, error)
	Decode([]byte) (T, error)
}
type JSON[T any] struct{}

func (JSON[T]) Encode(v T) ([]byte, error) { return json.Marshal(v) }
func (JSON[T]) Decode(b []byte) (T, error) { var v T; e := json.Unmarshal(b, &v); return v, e }

// Observer receives low-cardinality operation names, never keys or payloads.
// Implementations must be safe for concurrent calls and must not block.
type Observer interface {
	Observe(operation string, duration time.Duration, err error)
}

func Observe(o Observer, op string, start time.Time, err error) {
	if o != nil {
		o.Observe(op, time.Since(start), err)
	}
}
