# observability

**What it does:** Optional OpenTelemetry metrics and Gin tracing, using your providers. **When to use it:** Measuring request/dependency duration and cache outcomes.

`import "github.com/abdelrahmanAhmed1x/core/observability"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:observability -->
```go
metrics, err := observability.New(provider.Meter("shop"))
if err != nil {
	return err
}
r.Use(metrics.Middleware(tracer, propagation.TraceContext{}))
local := memory.New[Product](memory.Config{Entries: 1000, Observer: metrics}, nil)
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(metric.Meter)` | Create instruments; returns (*Instruments, error). |
| `Middleware / Record` | Measure Gin requests / your own named dependency calls. |
| `Observe` | Implement cache.Observer; pass Instruments as a cache/loader observer. |

**Configuration:** No config struct. Supply your meter/tracer/propagator; omit middleware and leave observers nil to disable instrumentation. [All options](../docs/configuration.md#observability).

**Important notes:** Applications configure exporters. Never use tokens, user IDs, cache keys or raw URLs as metric labels. [More](../docs/operations.md#observability).
