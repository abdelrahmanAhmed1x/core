// Package observability provides optional OpenTelemetry API instrumentation.
// Applications own providers/exporters. No SDK or global provider is installed.
package observability

import (
	"context"
	"errors"
	"github.com/abdelrahmanAhmed1x/core/cache"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"time"
)

type Instruments struct {
	duration metric.Float64Histogram
	calls    metric.Int64Counter
	failures metric.Int64Counter
}

func New(m metric.Meter) (*Instruments, error) {
	d, e1 := m.Float64Histogram("core.operation.duration", metric.WithUnit("s"))
	c, e2 := m.Int64Counter("core.operation.calls")
	f, e3 := m.Int64Counter("core.operation.errors")
	if e := errors.Join(e1, e2, e3); e != nil {
		return nil, e
	}
	return &Instruments{d, c, f}, nil
}
func (i *Instruments) Record(ctx context.Context, component, operation string, d time.Duration, err error) {
	if i == nil {
		return
	}
	a := metric.WithAttributes(attribute.String("component", component), attribute.String("operation", operation))
	i.duration.Record(ctx, d.Seconds(), a)
	i.calls.Add(ctx, 1, a)
	if err != nil {
		i.failures.Add(ctx, 1, a)
	}
}

// Observe implements cache.Observer. Operation names are chosen by cache code;
// do not call Record with arbitrary keys, user IDs, or URLs.
func (i *Instruments) Observe(op string, d time.Duration, err error) {
	outcome := "ok"
	if errors.Is(err, cache.ErrMiss) {
		outcome = "miss"
		err = nil
	} else if err != nil {
		outcome = "error"
	}
	i.Record(context.Background(), "cache", op+"."+outcome, d, err)
}

// Middleware traces using route templates, never raw request URLs or queries.
func (i *Instruments) Middleware(tracer trace.Tracer, prop propagation.TextMapPropagator) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ctx := c.Request.Context()
		if prop != nil {
			ctx = prop.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))
		}
		var span trace.Span
		if tracer != nil {
			ctx, span = tracer.Start(ctx, "HTTP", trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		if span != nil {
			span.SetName(c.Request.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route), attribute.Int("http.response.status_code", c.Writer.Status()))
		}
		var err error
		if c.Writer.Status() >= http.StatusInternalServerError {
			err = errors.New("http server error")
		}
		i.Record(ctx, "http", route, time.Since(start), err)
	}
}
