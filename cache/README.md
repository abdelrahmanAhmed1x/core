# cache

**What it does:** Loads data from a cache or your source function when it is missing. **When to use it:** Read-heavy product/detail endpoints that should avoid repeated database queries.

`import "github.com/abdelrahmanAhmed1x/core/cache"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:loader -->
```go
products := cache.NewLoader[Product](local, cache.LoaderConfig{Coalesce: true})
product, err := products.GetOrLoad(ctx, "product:42", time.Minute, loadProduct)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `NewLoader[T](backend, LoaderConfig)` | Wrap any cache backend. |
| `GetOrLoad` | Check cache, load a missing value, save it and return it. |
| `Store / Invalidate` | Write an updated value / remove a value through this loader. |
| `Cache[T] / Codec[T] / JSON[T]` | Backend contract / custom encoding / default JSON encoding. |

**Configuration:** `Coalesce=false` by default; set true to share concurrent loads. `Timeout=5s`, `MaxConcurrent=128`, `Jitter=0`, `Strict=false`. Negative caching is off. [All options](../docs/configuration.md#cache).

**Important notes:** With `Coalesce:true`, 100 requests for one missing product can share one database call within that process. By default cache failures fall back to the source; `Strict:true` returns them. [More](../docs/caching.md#loading-missing-values).

## Write and invalidate

<!-- example:loader-writes -->
```go
if err := products.Store(ctx, "product:42", updated, time.Minute); err != nil {
	return err
}
return products.Invalidate(ctx, "product:42")
```
<!-- /example -->
