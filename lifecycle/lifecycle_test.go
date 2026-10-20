package lifecycle

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShutdown(t *testing.T) {
	m := New(time.Second)
	var order []int
	boom := errors.New("cleanup")
	m.Register("one", func(context.Context) error { order = append(order, 1); return nil })
	m.Register("two", func(context.Context) error { order = append(order, 2); return boom })
	if e := m.Shutdown(context.Background()); !errors.Is(e, boom) {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(order, []int{2, 1}) {
		t.Fatal(order)
	}
	if e := m.Register("late", func(context.Context) error { return nil }); e == nil {
		t.Fatal("late hook allowed")
	}
	if e := m.Shutdown(context.Background()); !errors.Is(e, boom) {
		t.Fatal(e)
	}
}
func TestConcurrent(t *testing.T) {
	m := New(time.Second)
	var n atomic.Int32
	m.Register("once", func(context.Context) error { n.Add(1); return nil })
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if e := m.Shutdown(context.Background()); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
	if n.Load() != 1 {
		t.Fatal(n.Load())
	}
}
func TestDeadlineAndPanic(t *testing.T) {
	m := New(10 * time.Millisecond)
	m.Register("panic", func(context.Context) error { panic("secret") })
	m.Register("wait", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	if e := m.Shutdown(context.Background()); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}

func TestHTTPShutdown(t *testing.T) {
	m := New(time.Second)
	s := &http.Server{Addr: "127.0.0.1:0"}
	if e := m.RegisterHTTP("http", s); e != nil {
		t.Fatal(e)
	}
	if e := m.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := s.ListenAndServe(); !errors.Is(e, http.ErrServerClosed) {
		t.Fatal(e)
	}
}
