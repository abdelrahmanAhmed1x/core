# cache/redis

**What it does:** Shares cached values through Redis or DragonflyDB. **When to use it:** Multiple Go instances that need the same cached data without a local copy.

`import rediscache "github.com/abdelrahmanAhmed1x/core/cache/redis"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:redis-cache -->
```go
shared := rediscache.New[Product](rdb, rediscache.Config{
	Namespace: "shop:products:v1", Timeout: time.Second,
}, nil)
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](client, Config, codec)` | Use an existing redis.UniversalClient; nil codec selects JSON. |
| `Get / Set / Delete` | Use the same typed cache methods as memory. |

**Configuration:** `Namespace` is required. `Timeout=2s`, `MaxBytes=1MiB`, `Observer=nil`. Keys are stored under your namespace. [All options](../../docs/configuration.md#cache-redis).

**Important notes:** A miss is `cache.ErrMiss`; a backend error remains an error. TTL must be at least `1ms`. The application owns and closes the client. [More](../../docs/caching.md#redis-and-dragonflydb).
