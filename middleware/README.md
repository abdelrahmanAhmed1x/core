# middleware

**What it does:** Gin helpers for request IDs, body limits, response headers, CORS and deadlines. **When to use it:** The shared HTTP boundary of your API.

`import "github.com/abdelrahmanAhmed1x/core/middleware"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:middleware -->
```go
if err := r.SetTrustedProxies(nil); err != nil {
	return err
}
r.Use(middleware.RequestID(), middleware.SecurityHeaders())
r.Use(middleware.MaxBodySize(1<<20), middleware.Timeout(3*time.Second))
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `RequestID / SecurityHeaders` | Attach correlation IDs / common security headers. |
| `MaxBodySize / Timeout` | Limit reads / set a request context deadline. |
| `CORS(CORSConfig)` | Configure browser origins, methods, headers and credentials. |

**Configuration:** Body bytes and timeout must be positive. CORS requires `AllowedOrigins`; common options are `AllowedMethods`, `AllowedHeaders`, `AllowCredentials`, and `MaxAge` (seconds). [All options](../docs/configuration.md#middleware).

**Important notes:** `Timeout` needs handlers to honor context; it does not force a 504. Wildcard CORS cannot allow credentials. Configure trusted proxies for your ingress. [More](../docs/operations.md#http-middleware).

## Browser CORS

<!-- example:cors -->
```go
r.Use(middleware.CORS(middleware.CORSConfig{
	AllowedOrigins: []string{"https://app.example.com"},
	AllowedMethods: []string{"GET", "POST", "OPTIONS"},
	AllowedHeaders: []string{"Content-Type", "Authorization"}, AllowCredentials: true,
}))
```
<!-- /example -->
