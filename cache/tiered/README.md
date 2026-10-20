# cache/tiered

**What it does:** Checks memory first, then Redis/DragonflyDB. **When to use it:** Several instances serving hot data that can tolerate briefly outdated local copies.

`import "github.com/abdelrahmanAhmed1x/core/cache/tiered"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:tiered -->
```go
products := tiered.New[Product](local, shared, 2*time.Second, nil)
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](l1, l2, maxL1TTL, observer)` | Combine two existing caches; observer may be nil. |
| `Get / Set / Delete` | Read through both levels / write / remove from this instance’s levels. |

**Configuration:** Supply a positive `maxL1TTL`, such as `2*time.Second`. Configure capacity and connection settings on the underlying caches. [All options](../../docs/configuration.md#cache-tiered).

**Important notes:** **Other instances may return an older L1 copy until its TTL expires.** Choose Redis-only caching when you want to avoid this extra local staleness. [More](../../docs/caching.md#tiered-cache).
