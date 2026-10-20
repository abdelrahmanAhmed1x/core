package cache_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/cache"
	"github.com/abdelrahmanAhmed1x/core/cache/memory"
	cr "github.com/abdelrahmanAhmed1x/core/cache/redis"
	"github.com/abdelrahmanAhmed1x/core/cache/tiered"
	"github.com/abdelrahmanAhmed1x/core/internal/testredis"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mem[T any](n int) *memory.Cache[T] { return memory.New[T](memory.Config{Entries: n}, nil) }
func contract(t *testing.T, c cache.Cache[int]) {
	t.Helper()
	ctx := context.Background()
	if _, e := c.Get(ctx, "missing"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal(e)
	}
	if e := c.Set(ctx, "key", 7, 80*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if v, e := c.Get(ctx, "key"); e != nil || v.Value != 7 || time.Until(v.ExpiresAt) <= 0 {
		t.Fatal(v, e)
	}
	if e := c.Set(ctx, "bad", 1, 0); !errors.Is(e, cache.ErrTTL) {
		t.Fatal(e)
	}
	time.Sleep(100 * time.Millisecond)
	if _, e := c.Get(ctx, "key"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal(e)
	}
	if e := c.Set(ctx, "key", 8, time.Second); e != nil {
		t.Fatal(e)
	}
	if e := c.Delete(ctx, "key"); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Get(ctx, "key"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if e := c.Set(canceled, "c", 1, time.Second); e == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestMemory(t *testing.T) {
	contract(t, mem[int](2))
	c := mem[[]int](1)
	ctx := context.Background()
	v := []int{1}
	if e := c.Set(ctx, "a", v, time.Minute); e != nil {
		t.Fatal(e)
	}
	v[0] = 9
	out, e := c.Get(ctx, "a")
	if e != nil || out.Value[0] != 1 {
		t.Fatal(out, e)
	}
	if e = c.Set(ctx, "b", nil, time.Minute); e != nil {
		t.Fatal(e)
	}
	if _, e = c.Get(ctx, "a"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal("capacity not bounded", e)
	}
	out, e = c.Get(ctx, "b")
	if e != nil || out.Value != nil {
		t.Fatal("null not preserved", out, e)
	}
}
func TestRedis(t *testing.T) {
	r := testredis.Client(t)
	c := cr.New[int](r, cr.Config{Namespace: fmt.Sprintf("test-%d", time.Now().UnixNano())}, nil)
	contract(t, c)
	if e := r.Set(context.Background(), "corrupt:k", "no JSON", time.Minute).Err(); e != nil {
		t.Fatal(e)
	}
	bad := cr.New[int](r, cr.Config{Namespace: "corrupt"}, nil)
	if _, e := bad.Get(context.Background(), "k"); e == nil || errors.Is(e, cache.ErrMiss) {
		t.Fatal("corruption hidden", e)
	}
}
func TestTieredExpiry(t *testing.T) {
	l1, l2 := mem[int](2), mem[int](2)
	c := tiered.New[int](l1, l2, time.Second, nil)
	ctx := context.Background()
	if e := l2.Set(ctx, "k", 1, 100*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Get(ctx, "k"); e != nil {
		t.Fatal(e)
	}
	time.Sleep(130 * time.Millisecond)
	if _, e := c.Get(ctx, "k"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal("L1 extended L2 freshness", e)
	}
	if e := c.Set(ctx, "k", 2, time.Second); e != nil {
		t.Fatal(e)
	}
	if e := c.Delete(ctx, "k"); e != nil {
		t.Fatal(e)
	}
	if _, e := l2.Get(ctx, "k"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal(e)
	}
}

type failing struct{ getErr, setErr error }

func (f failing) Get(context.Context, string) (cache.Entry[int], error) {
	return cache.Entry[int]{}, f.getErr
}
func (f failing) Set(context.Context, string, int, time.Duration) error { return f.setErr }
func (f failing) Delete(context.Context, string) error                  { return nil }
func TestLoaderFailures(t *testing.T) {
	boom := errors.New("backend")
	for _, strict := range []bool{false, true} {
		l := cache.NewLoader[int](failing{boom, boom}, cache.LoaderConfig{Strict: strict})
		v, e := l.GetOrLoad(context.Background(), "k", time.Second, func(context.Context) (int, error) { return 7, nil })
		if strict && !errors.Is(e, boom) {
			t.Fatal(e)
		}
		if !strict && (e != nil || v != 7) {
			t.Fatal(v, e)
		}
	}
	l := cache.NewLoader[int](mem[int](2), cache.LoaderConfig{})
	if _, e := l.GetOrLoad(context.Background(), "k", time.Second, func(context.Context) (int, error) { return 0, boom }); !errors.Is(e, boom) {
		t.Fatal(e)
	}
	nilLoader := cache.NewLoader[any](mem[any](2), cache.LoaderConfig{Coalesce: true})
	v, e := nilLoader.GetOrLoad(context.Background(), "nil", time.Second, func(context.Context) (any, error) { return nil, nil })
	if e != nil || v != nil {
		t.Fatal(v, e)
	}
}
func TestCoalescingCancellation(t *testing.T) {
	l := cache.NewLoader[int](mem[int](4), cache.LoaderConfig{Coalesce: true})
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	load := func(context.Context) (int, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return 7, nil
	}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	wg.Go(func() {
		_, e := l.GetOrLoad(ctx, "key", time.Second, load)
		if !errors.Is(e, context.Canceled) {
			t.Error(e)
		}
	})
	<-started
	cancel()
	for range 20 {
		wg.Go(func() {
			v, e := l.GetOrLoad(context.Background(), "key", time.Second, load)
			if e != nil || v != 7 {
				t.Error(v, e)
			}
		})
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}
func TestNegativeAndConcurrency(t *testing.T) {
	l := cache.NewLoader[int](mem[int](128), cache.LoaderConfig{Coalesce: true, NegativeTTL: time.Second, NegativeEntries: 2})
	var calls atomic.Int32
	missing := func(context.Context) (int, error) { calls.Add(1); return 0, cache.ErrNotFound }
	for range 2 {
		if _, e := l.GetOrLoad(context.Background(), "missing", time.Second, missing); !errors.Is(e, cache.ErrNotFound) {
			t.Fatal(e)
		}
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	if e := l.Invalidate(context.Background(), "missing"); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			k := fmt.Sprint(i)
			_, e := l.GetOrLoad(context.Background(), k, time.Second, func(context.Context) (int, error) { return i, nil })
			if e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
}
func TestLoaderBound(t *testing.T) {
	l := cache.NewLoader[int](mem[int](4), cache.LoaderConfig{Coalesce: true, MaxConcurrent: 1})
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.GetOrLoad(context.Background(), "a", time.Second, func(context.Context) (int, error) { close(started); <-release; return 1, nil })
	}()
	<-started
	if _, e := l.GetOrLoad(context.Background(), "b", time.Second, func(context.Context) (int, error) { return 2, nil }); !errors.Is(e, cache.ErrBusy) {
		t.Fatal(e)
	}
	close(release)
	<-done
}
func FuzzJSON(f *testing.F) {
	f.Add([]byte(`{"x":1}`))
	f.Fuzz(func(t *testing.T, b []byte) { cache.JSON[map[string]int]{}.Decode(b) })
}
func BenchmarkMemoryHit(b *testing.B) {
	c := mem[int](100)
	ctx := context.Background()
	c.Set(ctx, "k", 1, time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Get(ctx, "k")
	}
}
func BenchmarkMemoryMiss(b *testing.B) {
	c := mem[int](100)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		c.Get(ctx, "missing")
	}
}
func BenchmarkMemorySet(b *testing.B) {
	c := mem[int](100)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		c.Set(ctx, "k", 1, time.Hour)
	}
}
func BenchmarkMemoryDelete(b *testing.B) {
	c := mem[int](100)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		c.Delete(ctx, "k")
	}
}
func BenchmarkJSON(b *testing.B) {
	codec := cache.JSON[map[string]string]{}
	v := map[string]string{"name": "example"}
	b.ReportAllocs()
	for b.Loop() {
		raw, _ := codec.Encode(v)
		codec.Decode(raw)
	}
}
func BenchmarkMemoryParallel(b *testing.B) {
	c := mem[int](100)
	ctx := context.Background()
	c.Set(ctx, "k", 1, time.Hour)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get(ctx, "k")
		}
	})
}
func BenchmarkRedis(b *testing.B) {
	r := testredis.Client(b)
	c := cr.New[int](r, cr.Config{Namespace: "core-bench"}, nil)
	ctx := context.Background()
	c.Set(ctx, "hit", 1, time.Hour)
	for _, key := range []string{"hit", "miss"} {
		b.Run(key, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, e := c.Get(ctx, key); e != nil && !errors.Is(e, cache.ErrMiss) {
					b.Fatal(e)
				}
			}
		})
	}
	b.Run("tiered", func(b *testing.B) {
		tc := tiered.New[int](mem[int](10), c, time.Minute, nil)
		b.ReportAllocs()
		for b.Loop() {
			tc.Get(ctx, "hit")
		}
	})
}
func BenchmarkCoalescedLoads(b *testing.B) {
	l := cache.NewLoader[int](failing{cache.ErrMiss, nil}, cache.LoaderConfig{Coalesce: true})
	ctx := context.Background()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.GetOrLoad(ctx, "same", time.Second, func(context.Context) (int, error) { return 1, nil })
		}
	})
}
func BenchmarkDifferentKeyLoads(b *testing.B) {
	l := cache.NewLoader[int](mem[int](1024), cache.LoaderConfig{Coalesce: true})
	ctx := context.Background()
	var id atomic.Int64
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprint(id.Add(1))
			l.GetOrLoad(ctx, key, time.Second, func(context.Context) (int, error) { return 1, nil })
		}
	})
}

func TestOrderedStore(t *testing.T) {
	ctx := context.Background()
	c := mem[int](4)
	l := cache.NewLoader[int](c, cache.LoaderConfig{Coalesce: true})
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.GetOrLoad(ctx, "key", time.Minute, func(context.Context) (int, error) { close(started); <-release; return 1, nil })
	}()
	<-started
	if e := l.Store(ctx, "key", 2, time.Minute); e != nil {
		t.Fatal(e)
	}
	close(release)
	<-done
	v, e := c.Get(ctx, "key")
	if e != nil || v.Value != 2 {
		t.Fatal("stale fill overwrote store", v, e)
	}
}

func TestLoaderTimeoutAndInvalidation(t *testing.T) {
	l := cache.NewLoader[int](mem[int](4), cache.LoaderConfig{Coalesce: true, Timeout: 10 * time.Millisecond})
	_, e := l.GetOrLoad(context.Background(), "t", time.Second, func(ctx context.Context) (int, error) { <-ctx.Done(); return 0, ctx.Err() })
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	c := mem[int](4)
	l = cache.NewLoader[int](c, cache.LoaderConfig{Coalesce: true})
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.GetOrLoad(context.Background(), "k", time.Minute, func(context.Context) (int, error) { close(started); <-release; return 1, nil })
	}()
	<-started
	if e = l.Invalidate(context.Background(), "k"); e != nil {
		t.Fatal(e)
	}
	close(release)
	<-done
	if _, e = c.Get(context.Background(), "k"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal("stale fill survived invalidation", e)
	}
}
func TestSizeAndJitter(t *testing.T) {
	c := memory.New[string](memory.Config{Entries: 2, MaxBytes: 4}, nil)
	if e := c.Set(context.Background(), "k", "too large", time.Second); !errors.Is(e, cache.ErrTooLarge) {
		t.Fatal(e)
	}
	under := mem[int](2)
	l := cache.NewLoader[int](under, cache.LoaderConfig{Jitter: 0.5})
	start := time.Now()
	if _, e := l.GetOrLoad(context.Background(), "k", time.Second, func(context.Context) (int, error) { return 1, nil }); e != nil {
		t.Fatal(e)
	}
	v, e := under.Get(context.Background(), "k")
	if e != nil || v.ExpiresAt.After(time.Now().Add(time.Second)) || v.ExpiresAt.Before(start.Add(500*time.Millisecond)) {
		t.Fatal(v, e)
	}
}
func TestLostInvalidationBound(t *testing.T) {
	ctx := context.Background()
	remote := mem[int](4)
	a := tiered.New[int](mem[int](4), remote, 40*time.Millisecond, nil)
	b := tiered.New[int](mem[int](4), remote, 40*time.Millisecond, nil)
	if e := a.Set(ctx, "k", 1, time.Second); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Get(ctx, "k"); e != nil {
		t.Fatal(e)
	}
	if e := a.Delete(ctx, "k"); e != nil {
		t.Fatal(e)
	}
	if v, e := b.Get(ctx, "k"); e != nil || v.Value != 1 {
		t.Fatal("expected documented stale copy", v, e)
	}
	time.Sleep(60 * time.Millisecond)
	if _, e := b.Get(ctx, "k"); !errors.Is(e, cache.ErrMiss) {
		t.Fatal("unbounded staleness", e)
	}
}

func BenchmarkRedisSetDelete(b *testing.B) {
	r := testredis.Client(b)
	c := cr.New[int](r, cr.Config{Namespace: "core-bench-mutations"}, nil)
	ctx := context.Background()
	b.Run("set", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if e := c.Set(ctx, "k", 1, time.Minute); e != nil {
				b.Fatal(e)
			}
		}
	})
	b.Run("set-delete", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if e := c.Set(ctx, "k", 1, time.Minute); e != nil {
				b.Fatal(e)
			}
			if e := c.Delete(ctx, "k"); e != nil {
				b.Fatal(e)
			}
		}
	})
}

func TestLoaderPanic(t *testing.T) {
	l := cache.NewLoader[int](mem[int](2), cache.LoaderConfig{Coalesce: true})
	_, e := l.GetOrLoad(context.Background(), "k", time.Second, func(context.Context) (int, error) { panic("sensitive panic") })
	if !errors.Is(e, cache.ErrLoaderPanic) {
		t.Fatal(e)
	}
}
