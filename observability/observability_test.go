package observability

import (
	"errors"
	"github.com/abdelrahmanAhmed1x/core/cache"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric/noop"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDisabledInstrumentation(t *testing.T) {
	i, e := New(noop.NewMeterProvider().Meter("test"))
	if e != nil {
		t.Fatal(e)
	}
	i.Observe("l1.get", time.Millisecond, cache.ErrMiss)
	i.Observe("l2.get", time.Millisecond, errors.New("backend"))
	r := gin.New()
	r.Use(i.Middleware(nooptrace.NewTracerProvider().Tracer("test"), nil))
	r.GET("/users/:id", func(c *gin.Context) { c.Status(204) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/users/secret?token=secret", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
