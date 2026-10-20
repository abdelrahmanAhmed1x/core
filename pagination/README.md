# pagination

**What it does:** Binds page/limit inputs and builds result metadata. **When to use it:** List endpoints using offset/limit database queries.

`import "github.com/abdelrahmanAhmed1x/core/pagination"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:pagination -->
```go
q, ok := httpx.BindQuery[pagination.Query](c)
if !ok {
	return
}
// Pass q.Offset() and q.Limit to your database query.
items, total := []Product{{ID: 42, Name: "Keyboard"}}, 1
httpx.OK(c, pagination.NewResult(items, total, q))
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `Query / EnsureDefaults / Offset` | Read page/limit / normalize direct inputs / compute an offset. |
| `NewResult[T] / Result[T] / Meta` | Wrap items with page, limit, total items and total pages. |

**Configuration:** Query defaults: `Page=1`, `Limit=10`, maximum limit `100`. No constructor config. [All options](../docs/configuration.md#pagination).

**Important notes:** Pass the real total count to NewResult. Empty results use an empty array. Very deep offsets can still be expensive for your database. [More](../docs/operations.md#pagination).
