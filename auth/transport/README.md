# auth/transport

**What it does:** Cookie/bearer extraction and secure cookie delivery shared by the auth packages. **When to use it:** Browser cookies, native bearer clients, or a custom authentication adapter.

`import "github.com/abdelrahmanAhmed1x/core/auth/transport"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:transport -->
```go
cookies := transport.Normalize(transport.Config{
	CookieName: "__Host-access", AllowedOrigins: []string{"https://app.example.com"},
})
transport.SetCookie(c.Writer, cookies, token, 15*time.Minute)
// To delete the client's cookie: transport.SetCookie(c.Writer, cookies, "", 0).
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `Normalize / ValidateOrigin` | Apply defaults and check cookie/origin configuration. |
| `Extract / SetCookie` | Read a credential / set or delete its cookie. |
| `Middleware[T]` | Adapt a context-based verifier to Gin. |
| `Payload[T] / StorePayload` | Read a trusted payload / populate it from a verified identity. |

**Configuration:** Defaults: `CookieName=__Host-session`, `Path=/`, Secure, HttpOnly, SameSite=Lax. Set exact `AllowedOrigins` for cookie-authenticated writes. [All options](../../docs/configuration.md#auth-transport).

**Important notes:** Conflicting cookie/bearer credentials are rejected. Deleting a cookie does not revoke server state. For local HTTP use an unprefixed name plus `InsecureDevelopment:true`. [More](../../docs/security.md#cookies-and-bearer-credentials).
