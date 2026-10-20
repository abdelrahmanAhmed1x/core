package health

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestReadiness(t *testing.T) {
	r := New(20 * time.Millisecond)
	r.Register("ok", func(context.Context) error { return nil })
	r.Register("bad", func(context.Context) error { return errors.New("secret://address") })
	r.Register("timeout", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	w := httptest.NewRecorder()
	r.Readiness(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 503 || w.Body.Len() != 0 {
		t.Fatal(w)
	}
	w = httptest.NewRecorder()
	r.Liveness(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestConcurrentChecks(t *testing.T) {
	r := New(time.Second)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for _, ok := range r.Ready(context.Background()) {
				if !ok {
					t.Error("false")
				}
			}
		})
	}
	r.Register("ok", func(context.Context) error { return nil })
	wg.Wait()
}

func TestGinHandlers(t *testing.T) {
	r := New(time.Second)
	g := gin.New()
	g.GET("/live", r.LiveHandler())
	g.GET("/ready", r.ReadyHandler())
	for _, path := range []string{"/live", "/ready"} {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
}
