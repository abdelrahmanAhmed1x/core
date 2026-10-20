# health

**What it does:** Separates process liveness from dependency readiness. **When to use it:** Load balancer or container health probes.

`import "github.com/abdelrahmanAhmed1x/core/health"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:health -->
```go
checks := health.New(time.Second)
checks.Register("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() })
r.GET("/live", checks.LiveHandler())
r.GET("/ready", checks.ReadyHandler())
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(timeout) / Register(name, Check)` | Create a registry / register a context-aware dependency check. |
| `Liveness / Readiness` | Standard HTTP handlers returning 200 or readiness 503. |
| `LiveHandler / ReadyHandler / Ready(ctx)` | Gin handlers / named results for private diagnostics. |

**Configuration:** Provide a positive readiness timeout. Each check returns an error when unhealthy. [All options](../docs/configuration.md#health).

**Important notes:** Liveness performs no external checks. Readiness hides infrastructure details. Checks must honor context. [More](../docs/operations.md#health-checks).
