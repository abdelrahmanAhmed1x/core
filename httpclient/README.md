# httpclient

**What it does:** Adds bounded retries and timeouts to standard outbound HTTP requests. **When to use it:** Calling an external API without repeating retry/backoff code.

`import "github.com/abdelrahmanAhmed1x/core/httpclient"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:httpclient -->
```go
client := httpclient.New(nil, httpclient.Config{Timeout: 3 * time.Second, Retries: 2})
req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
if err != nil {
	return err
}
resp, err := client.Do(req)
if err != nil {
	return err
}
defer resp.Body.Close()
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(client, Config)` | Wrap a caller-owned client, or pass nil for a standard transport. |
| `Do / CloseIdleConnections` | Send a request / explicitly close idle pooled connections. |

**Configuration:** `Timeout=10s`, `Retries=0`, `BaseBackoff=100ms`, `MaxBackoff=5s`. Retry status/error callbacks and request-ID propagation are optional. [All options](../docs/configuration.md#httpclient).

**Important notes:** Always close the final response body. Non-idempotent requests are not retried unless explicitly enabled; bodies must be replayable. [More](../docs/operations.md#outbound-http).
