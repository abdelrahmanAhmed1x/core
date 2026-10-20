# ratelimiter

**What it does:** Limits requests using a quota shared in Redis/DragonflyDB. **When to use it:** Protecting public endpoints or per-user API budgets.

`import "github.com/abdelrahmanAhmed1x/core/ratelimiter"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:ratelimiter -->
```go
limiter := ratelimiter.New(rdb)
r.Use(limiter.Middleware(ratelimiter.Limit{
	Rate: 100, Burst: 20, Period: time.Minute, FailureMode: ratelimiter.FailClosed,
}, ratelimiter.ByIP))
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(client)` | Use your existing Redis client. |
| `Middleware(Limit, KeyFunc)` | Apply a quota to routes. |
| `ByIP / KeyFunc` | Use client IP or supply a trusted user/tenant key. |

**Configuration:** `Rate`, `Burst`, and `Period` must be positive. Set `FailureMode:FailClosed` for 503 on store failure; the default `FailOpen` allows the request. [All options](../docs/configuration.md#ratelimiter).

**Important notes:** Exceeded quotas return 429 with Retry-After. Use trusted proxy settings for ByIP. The limiter never closes your client. [More](../docs/operations.md#rate-limiting).
