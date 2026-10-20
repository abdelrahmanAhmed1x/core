# lifecycle

**What it does:** Runs explicitly registered cleanup once, in reverse order. **When to use it:** Draining HTTP before closing shared clients during shutdown.

`import "github.com/abdelrahmanAhmed1x/core/lifecycle"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:lifecycle -->
```go
shutdown := lifecycle.New(10 * time.Second)
if err := shutdown.Register("redis", func(context.Context) error { return rdb.Close() }); err != nil {
	return err
}
if err := shutdown.RegisterHTTP("http", server); err != nil {
	return err
}
// Call after receiving a shutdown signal, using a fresh context:
return shutdown.Shutdown(context.Background())
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(timeout)` | Set the overall shutdown budget. |
| `Register / RegisterHTTP` | Add named cleanup / standard HTTP graceful shutdown. |
| `Shutdown(ctx)` | Run hooks and return joined cleanup errors; repeated calls do not rerun them. |

**Configuration:** Provide a positive timeout. Register dependencies first and HTTP last. [All options](../docs/configuration.md#lifecycle).

**Important notes:** Use a fresh context after signal cancellation. Cleanup hooks must honor their deadline; Core cannot stop a stuck function. [More](../docs/operations.md#shutdown).
