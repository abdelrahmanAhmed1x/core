package cache

import (
	"context"
	"errors"
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// LoaderConfig defaults to availability: lookup/decode/write errors are observed
// and a successful source result survives a failed cache write. Strict opts out.
// NegativeTTL only caches ErrNotFound locally, with bounded NegativeEntries.
type LoaderConfig struct {
	Strict          bool
	Coalesce        bool
	Timeout         time.Duration
	MaxConcurrent   int
	Jitter          float64
	NegativeTTL     time.Duration
	NegativeEntries int
	Observer        Observer
}
type stripe struct {
	sync.Mutex
	generation uint64
}
type loaded[T any] struct{ value T }
type Loader[T any] struct {
	cache    Cache[T]
	cfg      LoaderConfig
	group    singleflight.Group
	stripes  [64]stripe
	slots    chan struct{}
	negative *lru.Cache[string, time.Time]
}

func NewLoader[T any](c Cache[T], cfg LoaderConfig) *Loader[T] {
	if c == nil || cfg.Timeout < 0 || cfg.MaxConcurrent < 0 || math.IsNaN(cfg.Jitter) || cfg.Jitter < 0 || cfg.Jitter >= 1 || cfg.NegativeTTL < 0 || cfg.NegativeEntries < 0 {
		panic("cache: invalid loader configuration")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 128
	}
	if cfg.NegativeTTL > 0 && cfg.NegativeEntries == 0 {
		cfg.NegativeEntries = 1024
	}
	l := &Loader[T]{cache: c, cfg: cfg, slots: make(chan struct{}, cfg.MaxConcurrent)}
	if cfg.NegativeTTL > 0 {
		l.negative, _ = lru.New[string, time.Time](cfg.NegativeEntries)
	}
	return l
}
func (l *Loader[T]) GetOrLoad(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	var z T
	if len(key) > 4096 {
		return z, ErrTooLarge
	}
	if e := ctx.Err(); e != nil {
		return z, e
	}
	if ttl <= 0 {
		return z, ErrTTL
	}
	if load == nil {
		return z, errors.New("cache: nil loader")
	}
	if l.negative != nil {
		if until, ok := l.negative.Get(key); ok && time.Now().Before(until) {
			return z, ErrNotFound
		}
	}
	v, e := l.cache.Get(ctx, key)
	if e == nil {
		return v.Value, nil
	}
	if !errors.Is(e, ErrMiss) {
		Observe(l.cfg.Observer, "cache.error", time.Now(), e)
		if l.cfg.Strict {
			return z, e
		}
	}
	run := func() (any, error) {
		select {
		case l.slots <- struct{}{}:
			defer func() { <-l.slots }()
		default:
			return z, ErrBusy
		}
		// Coalesced work is independent of any single waiter but has a finite deadline.
		work := ctx
		if l.cfg.Coalesce {
			work = context.WithoutCancel(ctx)
		}
		work, cancel := context.WithTimeout(work, l.cfg.Timeout)
		defer cancel()
		s := l.stripe(key)
		s.Lock()
		generation := s.generation
		s.Unlock()
		if v, e := l.cache.Get(work, key); e == nil {
			return loaded[T]{v.Value}, nil
		} else if !errors.Is(e, ErrMiss) {
			Observe(l.cfg.Observer, "cache.error", time.Now(), e)
			if l.cfg.Strict {
				return z, e
			}
		}
		if e := work.Err(); e != nil {
			return z, e
		}
		start := time.Now()
		value, e := callLoader(work, load)
		Observe(l.cfg.Observer, "loader", start, e)
		if e == nil {
			e = work.Err()
		}
		if e != nil {
			if errors.Is(e, ErrNotFound) && l.negative != nil {
				s.Lock()
				if s.generation == generation {
					l.negative.Add(key, time.Now().Add(l.cfg.NegativeTTL))
				}
				s.Unlock()
			}
			return z, e
		}
		actual := ttl
		if l.cfg.Jitter > 0 {
			actual = time.Duration(float64(ttl) * (1 - rand.Float64()*l.cfg.Jitter))
			if actual <= 0 {
				actual = 1
			}
		}
		s.Lock()
		if s.generation == generation {
			e = l.cache.Set(work, key, value, actual)
		}
		s.Unlock()
		Observe(l.cfg.Observer, "cache.write", time.Now(), e)
		if e != nil && l.cfg.Strict {
			return z, e
		}
		return loaded[T]{value}, nil
	}
	if !l.cfg.Coalesce {
		v, e := run()
		if e != nil {
			return z, e
		}
		return v.(loaded[T]).value, nil
	}
	ch := l.group.DoChan(key, run)
	select {
	case <-ctx.Done():
		return z, ctx.Err()
	case r := <-ch:
		if r.Shared {
			Observe(l.cfg.Observer, "singleflight.shared", time.Now(), r.Err)
		}
		if r.Err != nil {
			return z, r.Err
		}
		return r.Val.(loaded[T]).value, nil
	}
}

// Invalidate clears this loader's negative cache and deletes the backend value.
// Applications must order invalidation against concurrent source writes/loads.
func (l *Loader[T]) Invalidate(ctx context.Context, key string) error {
	s := l.stripe(key)
	s.Lock()
	defer s.Unlock()
	s.generation++
	l.group.Forget(key)
	if l.negative != nil {
		l.negative.Remove(key)
	}
	return l.cache.Delete(ctx, key)
}

func (l *Loader[T]) stripe(key string) *stripe {
	h := fnv.New64a()
	h.Write([]byte(key))
	return &l.stripes[h.Sum64()%uint64(len(l.stripes))]
}

// Store orders local writes against this loader's in-flight fills. Applications
// must use Store/Invalidate on the same Loader to obtain that guarantee. Other
// processes and direct backend writes require application-level versioning.
func (l *Loader[T]) Store(ctx context.Context, key string, value T, ttl time.Duration) error {
	if len(key) > 4096 {
		return ErrTooLarge
	}
	if ttl <= 0 {
		return ErrTTL
	}
	s := l.stripe(key)
	s.Lock()
	defer s.Unlock()
	s.generation++
	l.group.Forget(key)
	if l.negative != nil {
		l.negative.Remove(key)
	}
	return l.cache.Set(ctx, key, value, ttl)
}

func callLoader[T any](ctx context.Context, load func(context.Context) (T, error)) (v T, err error) {
	defer func() {
		if recover() != nil {
			err = ErrLoaderPanic
		}
	}()
	return load(ctx)
}
