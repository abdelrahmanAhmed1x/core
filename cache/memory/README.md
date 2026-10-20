# cache/memory

**What it does:** Stores data inside this Go process with a bounded entry count. **When to use it:** One-server applications or a local cache of frequently read data.

`import "github.com/abdelrahmanAhmed1x/core/cache/memory"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:memory -->
```go
local := memory.New[Product](memory.Config{Entries: 1000}, nil) // nil selects JSON.
if err := local.Set(ctx, "product:42", product, time.Minute); err != nil {
	return err
}
entry, err := local.Get(ctx, "product:42")
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](Config, codec)` | Create the cache; nil codec selects JSON. |
| `Get / Set / Delete` | Read Entry[T]{Value, ExpiresAt} / write with TTL / remove. |
| `Purge` | Remove all entries from this cache instance. |

**Configuration:** `Entries` is required. `MaxBytes=1MiB` limits encoded values; `Namespace` is optional; `Observer=nil` disables metrics. [All options](../../docs/configuration.md#cache-memory).

**Important notes:** `Get` returns `cache.ErrMiss` when absent or expired. TTL must be positive. Instances do not share contents; there are no workers or Close method. [More](../../docs/caching.md#memory-cache).
