# Core

Independent Go packages for authentication, caching, HTTP APIs, and service utilities.
Import only what your application needs. Requires Go 1.26.5 or later; use a patched toolchain for deployment.

```sh
go get github.com/abdelrahmanAhmed1x/core
```

## Packages

| Package | What it does | When to use it |
|---|---|---|
| [`auth/stateful`](#auth-stateful) | Opaque stored sessions | Web/mobile logout and revocation |
| [`auth/jwt`](#auth-jwt) | JWT access + stored refresh | JWT APIs with persistent login |
| [`auth/stateless`](#auth-stateless) | JWT access + JWT refresh | No store; accept revocation limits |
| [`auth/single`](#auth-single) | One expiring JWT | Small APIs without refresh |
| [`auth/transport`](#auth-transport) | Cookie/bearer helpers | Web/mobile credential delivery |
| [`cache`](#cache) | Cache loaders and codecs | Avoid repeated source queries |
| [`cache/memory`](#cache-memory) | In-process cache | One process or local hot data |
| [`cache/redis`](#cache-redis) | Redis/DragonflyDB cache | Share values across instances |
| [`cache/tiered`](#cache-tiered) | Memory then Redis | Fast reads with brief local staleness |
| [`authorization`](#authorization) | Roles and permissions | Protected application actions |
| [`httpx`](#httpx) | Binding and JSON responses | Consistent Gin handlers |
| [`logger`](#logger) | Contextual slog logging | Service/access logs |
| [`middleware`](#middleware) | CORS, IDs, limits, deadlines | Shared API request handling |
| [`pagination`](#pagination) | Page/limit helpers | Paginated list endpoints |
| [`ratelimiter`](#ratelimiter) | Shared request quotas | Abuse protection |
| [`health`](#health) | Liveness/readiness | Deployment health probes |
| [`httpclient`](#httpclient) | HTTP retry and timeout helpers | External API calls |
| [`observability`](#observability) | Optional OTel instrumentation | Metrics and tracing |
| [`lifecycle`](#lifecycle) | Ordered cleanup | Graceful service shutdown |

## Quick start

The snippets are fragments for your existing application, not separate servers.
Import the package named in each section under `github.com/abdelrahmanAhmed1x/core/`.
Use alias `hybrid` for `auth/jwt` and `rediscache` for `cache/redis`.
Shared standard imports are `context`, `time`, and (for outbound HTTP) `net/http`;
Gin routes use `github.com/gin-gonic/gin`.

<!-- example:models -->
```go
type User struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}
type Product struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
```
<!-- /example -->

<!-- example:setup -->
```go
ctx := context.Background() // In a handler, use c.Request.Context().
r := gin.New()
r.Use(gin.Recovery())
```
<!-- /example -->

Redis-backed examples use a caller-owned `rdb` (`github.com/redis/go-redis/v9`):

<!-- example:redis-client -->
```go
rdb := redis.NewClient(&redis.Options{
	Addr: "127.0.0.1:6379", ContextTimeoutEnabled: true,
	DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
})
defer rdb.Close() // In main: the application owns this shared client.
```
<!-- /example -->

Use a local Redis/DragonflyDB server at `127.0.0.1:6379` for those examples.
`signingKey` and `refreshKey` come from your secret manager (random, at least 32 bytes;
distinct for stateless pairs). `verifiedUser` means your login flow has **already
verified identity**. Never issue credentials from unchecked client input.
Replace the example user ID `user-42` with that verified user’s ID.

**Cache a product.** `loadProduct` is your `func(context.Context) (Product, error)`
that fetches product 42 from the database:

<!-- example:quick-cache -->
```go
local := memory.New[Product](memory.Config{Entries: 1000}, nil)
products := cache.NewLoader[Product](local, cache.LoaderConfig{Coalesce: true})
product, err := products.GetOrLoad(ctx, "product:42", time.Minute, loadProduct)
if err != nil {
	return err
}
```
<!-- /example -->

**Create a JWT manager** (no Redis needed):

<!-- example:quick-jwt -->
```go
auth := single.New[User](single.Config{
	SecretKey: signingKey, Issuer: "shop", Audience: "api", TTL: 15 * time.Minute,
})
```
<!-- /example -->

**Add a shared request quota** to the Gin router:

<!-- example:ratelimiter -->
```go
limiter := ratelimiter.New(rdb)
r.Use(limiter.Middleware(ratelimiter.Limit{
	Rate: 100, Burst: 20, Period: time.Minute, FailureMode: ratelimiter.FailClosed,
}, ratelimiter.ByIP))
```
<!-- /example -->

See [stateful auth](#auth-stateful) for revocable sessions. Each package section
below starts with its constructor and a focused usage example.
Runnable programs: [auth](examples/auth/main.go), [cache](examples/cache/main.go),
[HTTP service with shutdown](examples/server/main.go). Run `go run ./examples/cache`
without Redis. All marked snippets are [compile checked](examples/handbook/handbook_test.go).

## Authentication

| Strategy | Credential | Store | Logout/revocation | Refresh |
|---|---|---|---|---|
| Stateful | Opaque session | Redis/DragonflyDB | Yes | Idle renewal |
| Hybrid | JWT access, opaque refresh | Redis/DragonflyDB | Refresh state only | Yes |
| Stateless | Two JWTs | None | No individual revocation | Yes, reusable |
| Single | One JWT | None | No individual revocation | None |

All four managers protect Gin routes and expose the authenticated user the same way:

<!-- example:auth-route -->
```go
r.GET("/me", auth.Middleware(), func(c *gin.Context) {
	user, ok := auth.Payload(c)
	if !ok {
		c.AbortWithStatus(401)
		return
	}
	c.JSON(200, user)
})
```
<!-- /example -->

Clients can send `Authorization: Bearer <credential>`. Browsers can use secure cookies;
set exact allowed origins for cookie-authenticated writes. Use `c.Request.Context()`
in handlers. Application permissions are separate from login.

<a id="auth-stateful"></a>

### `auth/stateful`

**What it does:** Opaque login sessions stored in Redis or DragonflyDB. Sessions support logout and revocation. **When to use it:** Web/mobile applications where users need device logout or logout-all.

**How to use it:**

<!-- example:stateful -->
```go
auth := stateful.New[User](rdb, stateful.Config{
	Namespace: "shop:sessions", AbsoluteTTL: 24 * time.Hour,
})
token, err := auth.Create(ctx, "user-42", verifiedUser)
if err != nil {
	return err
}
user, err := auth.Verify(ctx, token)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](client, Config)` | Create a session manager. |
| `Create / Verify` | Issue a session credential / look it up. |
| `Renew / Revoke / RevokeAll` | Extend idle lifetime / revoke one session / revoke a user’s sessions. |
| `Login / Logout` | Set or clear browser credentials and update stored sessions. |
| `Middleware / OptionalMiddleware / Payload` | Require login / allow guests / read the typed user. |

**Configuration:** `Namespace` and `AbsoluteTTL` are required. `IdleTTL=0` disables idle expiry; `OperationTimeout` defaults to `3s`. `Transport` controls cookies. [All options](docs/configuration.md#auth-stateful).

**Important notes:** Protected requests fail when Redis is unavailable. Renewal never extends the original maximum session age. For browser writes, set `Transport.AllowedOrigins`; send bearer credentials from native clients. [More](docs/authentication.md#stateful-sessions).

<a id="auth-jwt"></a>

### `auth/jwt`

**What it does:** JWT access tokens with opaque refresh credentials stored in Redis/DragonflyDB. **When to use it:** Applications that need JWT access tokens and persistent login.

**How to use it:**

<!-- example:hybrid -->
```go
auth := hybrid.New[User](rdb, hybrid.Config{
	SecretKey: signingKey, Issuer: "shop", CookieSecure: true,
	AllowedOrigins: []string{"https://app.example.com"},
})
if err := auth.Login(c, verifiedUser); err != nil {
	return err
}
// In separate handlers: auth.Refresh(c) and auth.Logout(c), both return error.
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](client, Config)` | Create the hybrid Authenticator[T]. |
| `Login / Refresh / Logout` | Set credential cookies / refresh the pair / revoke refresh state and clear cookies. |
| `Middleware / OptionalMiddleware` | Verify access credentials / optionally authenticate. |
| `Payload / MustPayload` | Read the typed user; MustPayload panics if missing. |

**Configuration:** `SecretKey` and `Issuer` are required. `AccessTTL=15m`, `RefreshTTL=7d`; `Audience` is optional. Set `CookieSecure:true` and trusted `AllowedOrigins`. [All options](docs/configuration.md#auth-jwt).

**Important notes:** Logout revokes refresh state; existing access JWTs still work until expiry. This package’s operations use Gin contexts. Its optional middleware treats invalid single credentials as guests. [More](docs/authentication.md#hybrid-jwt).

<a id="auth-stateless"></a>

### `auth/stateless`

**What it does:** Signed access and refresh JWTs without a session store. **When to use it:** Services that accept reusable refresh credentials and no individual server-side revocation.

**How to use it:**

<!-- example:stateless -->
```go
auth := stateless.New[User](stateless.Config{
	Issuer: "shop", AccessAudience: "api", RefreshAudience: "refresh",
	AccessKey: signingKey, RefreshKey: refreshKey,
	AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
})
pair, err := auth.Issue(verifiedUser)
if err != nil {
	return err
}
user, err := auth.VerifyAccess(pair.Access)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](Config)` | Create the JWT pair manager. |
| `Issue` | Return Pair{Access, Refresh} after identity verification. |
| `VerifyAccess / VerifyRefresh` | Check the intended credential and return its payload. |
| `Refresh` | Issue another pair from a valid refresh JWT. |
| `Middleware / OptionalMiddleware / Payload` | Protect access routes / allow guests / read the user. |

**Configuration:** `Issuer`, distinct `AccessAudience`/`RefreshAudience`, distinct `AccessKey`/`RefreshKey`, and both TTLs are required. `Leeway=0`; `Transport` controls access cookies. [All options](docs/configuration.md#auth-stateless).

**Important notes:** **Neither JWT can be individually revoked before expiry. Refreshing does not invalidate the old pair.** Delete client credentials to sign out locally; there is no server-side Logout. [More](docs/authentication.md#stateless-pair).

<a id="auth-single"></a>

### `auth/single`

**What it does:** One expiring JWT, with no refresh token or datastore. **When to use it:** Small APIs and short-lived credentials where server-side revocation is unnecessary.

**How to use it:**

<!-- example:single -->
```go
auth := single.New[User](single.Config{
	SecretKey: signingKey, Issuer: "shop", Audience: "api", TTL: 15 * time.Minute,
})
token, err := auth.Issue(verifiedUser)
if err != nil {
	return err
}
user, err := auth.Verify(token)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](Config)` | Create the JWT manager. |
| `Issue / Verify` | Sign a credential / check it and return the payload. |
| `Middleware / OptionalMiddleware / Payload` | Require login / allow guests / read the typed user. |

**Configuration:** `SecretKey`, `Issuer`, `Audience`, and `TTL` are required. `Leeway=0`; `Transport` controls cookies. [All options](docs/configuration.md#auth-single).

**Important notes:** **A copied JWT remains valid until expiry**, even after local cookie deletion. There is no refresh or server-side Logout. [More](docs/authentication.md#single-jwt).

<a id="auth-transport"></a>

### `auth/transport`

**What it does:** Cookie/bearer extraction and secure cookie delivery shared by the auth packages. **When to use it:** Browser cookies, native bearer clients, or a custom authentication adapter.

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

**Configuration:** Defaults: `CookieName=__Host-session`, `Path=/`, Secure, HttpOnly, SameSite=Lax. Set exact `AllowedOrigins` for cookie-authenticated writes. [All options](docs/configuration.md#auth-transport).

**Important notes:** Conflicting cookie/bearer credentials are rejected. Deleting a cookie does not revoke server state. For local HTTP use an unprefixed name plus `InsecureDevelopment:true`. [More](docs/security.md#cookies-and-bearer-credentials).

## Caching

Memory keeps values in one process. Redis/DragonflyDB shares them across instances.
Tiered caching checks memory first, then the shared store. Use the same Get/Set/Delete
methods on any backend; a missing value is `cache.ErrMiss`, not a server failure.

<a id="cache"></a>

### `cache`

**What it does:** Loads data from a cache or your source function when it is missing. **When to use it:** Read-heavy product/detail endpoints that should avoid repeated database queries.

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

**Configuration:** `Coalesce=false` by default; set true to share concurrent loads. `Timeout=5s`, `MaxConcurrent=128`, `Jitter=0`, `Strict=false`. Negative caching is off. [All options](docs/configuration.md#cache).

**Important notes:** With `Coalesce:true`, 100 requests for one missing product can share one database call within that process. By default cache failures fall back to the source; `Strict:true` returns them. [More](docs/caching.md#loading-missing-values).

<a id="cache-memory"></a>

### `cache/memory`

**What it does:** Stores data inside this Go process with a bounded entry count. **When to use it:** One-server applications or a local cache of frequently read data.

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

**Configuration:** `Entries` is required. `MaxBytes=1MiB` limits encoded values; `Namespace` is optional; `Observer=nil` disables metrics. [All options](docs/configuration.md#cache-memory).

**Important notes:** `Get` returns `cache.ErrMiss` when absent or expired. TTL must be positive. Instances do not share contents; there are no workers or Close method. [More](docs/caching.md#memory-cache).

<a id="cache-redis"></a>

### `cache/redis`

**What it does:** Shares cached values through Redis or DragonflyDB. **When to use it:** Multiple Go instances that need the same cached data without a local copy.

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

**Configuration:** `Namespace` is required. `Timeout=2s`, `MaxBytes=1MiB`, `Observer=nil`. Keys are stored under your namespace. [All options](docs/configuration.md#cache-redis).

**Important notes:** A miss is `cache.ErrMiss`; a backend error remains an error. TTL must be at least `1ms`. The application owns and closes the client. [More](docs/caching.md#redis-and-dragonflydb).

<a id="cache-tiered"></a>

### `cache/tiered`

**What it does:** Checks memory first, then Redis/DragonflyDB. **When to use it:** Several instances serving hot data that can tolerate briefly outdated local copies.

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

**Configuration:** Supply a positive `maxL1TTL`, such as `2*time.Second`. Configure capacity and connection settings on the underlying caches. [All options](docs/configuration.md#cache-tiered).

**Important notes:** **Other instances may return an older L1 copy until its TTL expires.** Choose Redis-only caching when you want to avoid this extra local staleness. [More](docs/caching.md#tiered-cache).

## HTTP and service utilities

<a id="authorization"></a>

### `authorization`

**What it does:** Checks roles, permissions and your own policy functions against a trusted user. **When to use it:** Admin routes, permission checks, or application resource policies.

**How to use it:**

<!-- example:authorization -->
```go
access := authorization.New(func(c *gin.Context) (authorization.Principal, error) {
	user, ok := auth.Payload(c)
	if !ok {
		return authorization.Principal{}, authorization.ErrUnauthenticated
	}
	return authorization.Principal{Roles: []string{user.Role}}, nil
})
r.GET("/admin", auth.Middleware(), access.RequireRole("admin"), func(c *gin.Context) {
	c.Status(204)
})
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(Resolver)` | Resolve trusted roles/permissions, optionally from your database. |
| `RequireRole / RequireAnyRole / RequirePermission` | Allow matching roles or permissions. |
| `Require(Policy)` | Run your own context-aware predicate. |

**Configuration:** No config struct. A resolver returns `Principal{Roles, Permissions}` or an error. [All options](docs/configuration.md#authorization).

**Important notes:** Missing users should return `ErrUnauthenticated` (401); denied access is 403. Empty requirements deny. Token roles may be stale: load fresh permissions when needed. [More](docs/security.md#authorization).

<a id="httpx"></a>

### `httpx`

**What it does:** Typed Gin binding and consistent success/error JSON responses. **When to use it:** Handlers that should share request validation and response shapes.

**How to use it:**

<!-- example:httpx -->
```go
request, ok := httpx.BindJSON[struct {
	Name string `json:"name" binding:"required"`
}](c)
if !ok {
	return
} // A 400 response has already been sent.
httpx.Created(c, Product{ID: 42, Name: request.Name})
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `BindJSON[T] / BindURI[T] / BindQuery[T]` | Bind and validate input; return (value, ok). |
| `OK / Created / NoContent` | Send 200 / 201 / 204 responses. |
| `AbortBadRequest / AbortUnauthorized / AbortForbidden / AbortInternal` | Stop with safe HTTP errors; more Abort helpers are listed in the package README. |

**Configuration:** No constructor or config. Use Gin `json`, `uri`, `form`, and `binding` struct tags. [All options](docs/configuration.md#httpx).

**Important notes:** Return when binding reports false: the 400 response is already sent. Keep application-supplied error messages/details safe for clients. [More](docs/operations.md#http-responses).

<a id="logger"></a>

### `logger`

**What it does:** Contextual structured logging backed by standard slog. **When to use it:** JSON service logs and request-correlated Gin access logs.

**How to use it:**

<!-- example:logger -->
```go
log := logger.New(logger.Config{Format: "json", MaskSensitives: true})
r.Use(logger.Middleware(log))
log.Info(ctx, "service started", "component", "api")
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(Config)` | Create a logger. |
| `Debug / Info / Warn / Error / With` | Log with context and structured attributes / attach shared attributes. |
| `Middleware / ContextWithRequestID / Slog` | Access logs / correlation context / standard slog integration. |

**Configuration:** `Level=info`, `Format=text`, `Output=stdout`. Set `Format:"json"` and `MaskSensitives:true` to enable known-key redaction. [All options](docs/configuration.md#logger).

**Important notes:** Redaction matches attribute keys; it does not scrub arbitrary messages or error strings. Never put credentials there. [More](docs/security.md#logging).

<a id="middleware"></a>

### `middleware`

**What it does:** Gin helpers for request IDs, body limits, response headers, CORS and deadlines. **When to use it:** The shared HTTP boundary of your API.

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

**Configuration:** Body bytes and timeout must be positive. CORS requires `AllowedOrigins`; common options are `AllowedMethods`, `AllowedHeaders`, `AllowCredentials`, and `MaxAge` (seconds). [All options](docs/configuration.md#middleware).

**Important notes:** `Timeout` needs handlers to honor context; it does not force a 504. Wildcard CORS cannot allow credentials. Configure trusted proxies for your ingress. [More](docs/operations.md#http-middleware).

<a id="pagination"></a>

### `pagination`

**What it does:** Binds page/limit inputs and builds result metadata. **When to use it:** List endpoints using offset/limit database queries.

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

**Configuration:** Query defaults: `Page=1`, `Limit=10`, maximum limit `100`. No constructor config. [All options](docs/configuration.md#pagination).

**Important notes:** Pass the real total count to NewResult. Empty results use an empty array. Very deep offsets can still be expensive for your database. [More](docs/operations.md#pagination).

<a id="ratelimiter"></a>

### `ratelimiter`

**What it does:** Limits requests using a quota shared in Redis/DragonflyDB. **When to use it:** Protecting public endpoints or per-user API budgets.

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

**Configuration:** `Rate`, `Burst`, and `Period` must be positive. Set `FailureMode:FailClosed` for 503 on store failure; the default `FailOpen` allows the request. [All options](docs/configuration.md#ratelimiter).

**Important notes:** Exceeded quotas return 429 with Retry-After. Use trusted proxy settings for ByIP. The limiter never closes your client. [More](docs/operations.md#rate-limiting).

<a id="health"></a>

### `health`

**What it does:** Separates process liveness from dependency readiness. **When to use it:** Load balancer or container health probes.

**How to use it:**

<!-- example:health -->
```go
checks := health.New(time.Second)
checks.Register("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() })
r.GET("/live", checks.LiveHandler())
r.GET("/ready", checks.ReadyHandler())
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(timeout) / Register(name, Check)` | Create a registry / register a context-aware dependency check. |
| `Liveness / Readiness` | Standard HTTP handlers returning 200 or readiness 503. |
| `LiveHandler / ReadyHandler / Ready(ctx)` | Gin handlers / named results for private diagnostics. |

**Configuration:** Provide a positive readiness timeout. Each check returns an error when unhealthy. [All options](docs/configuration.md#health).

**Important notes:** Liveness performs no external checks. Readiness hides infrastructure details. Checks must honor context. [More](docs/operations.md#health-checks).

<a id="httpclient"></a>

### `httpclient`

**What it does:** Adds bounded retries and timeouts to standard outbound HTTP requests. **When to use it:** Calling an external API without repeating retry/backoff code.

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

**Configuration:** `Timeout=10s`, `Retries=0`, `BaseBackoff=100ms`, `MaxBackoff=5s`. Retry status/error callbacks and request-ID propagation are optional. [All options](docs/configuration.md#httpclient).

**Important notes:** Always close the final response body. Non-idempotent requests are not retried unless explicitly enabled; bodies must be replayable. [More](docs/operations.md#outbound-http).

<a id="observability"></a>

### `observability`

**What it does:** Optional OpenTelemetry metrics and Gin tracing, using your providers. **When to use it:** Measuring request/dependency duration and cache outcomes.

**How to use it:**

<!-- example:observability -->
```go
metrics, err := observability.New(provider.Meter("shop"))
if err != nil {
	return err
}
r.Use(metrics.Middleware(tracer, propagation.TraceContext{}))
local := memory.New[Product](memory.Config{Entries: 1000, Observer: metrics}, nil)
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(metric.Meter)` | Create instruments; returns (*Instruments, error). |
| `Middleware / Record` | Measure Gin requests / your own named dependency calls. |
| `Observe` | Implement cache.Observer; pass Instruments as a cache/loader observer. |

**Configuration:** No config struct. Supply your meter/tracer/propagator; omit middleware and leave observers nil to disable instrumentation. [All options](docs/configuration.md#observability).

**Important notes:** Applications configure exporters. Never use tokens, user IDs, cache keys or raw URLs as metric labels. [More](docs/operations.md#observability).

<a id="lifecycle"></a>

### `lifecycle`

**What it does:** Runs explicitly registered cleanup once, in reverse order. **When to use it:** Draining HTTP before closing shared clients during shutdown.

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

**Configuration:** Provide a positive timeout. Register dependencies first and HTTP last. [All options](docs/configuration.md#lifecycle).

**Important notes:** Use a fresh context after signal cancellation. Cleanup hooks must honor their deadline; Core cannot stop a stuck function. [More](docs/operations.md#shutdown).

## Further reading

- [Authentication](docs/authentication.md): browser/mobile recipes, refresh behavior, migration.
- [Caching](docs/caching.md): backend choice, errors, concurrency and invalidation.
- [Configuration](docs/configuration.md): complete field/default reference.
- [Security](docs/security.md): credentials, cookies, authorization and logging boundaries.
- [Operations](docs/operations.md): Redis, HTTP, health, telemetry and shutdown.
- [Benchmarks](docs/benchmarks.md): recorded measurements and how to reproduce them.

Run `go test ./...` for unit/documentation examples; see the operations guide for
Redis/DragonflyDB integration tests. Core does not create or close your dependencies
unless your application explicitly registers cleanup. Invalid static configuration
usually panics at construction; normal operational failures return errors.
