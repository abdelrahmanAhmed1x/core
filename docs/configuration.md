# Configuration reference

Use the README examples first; this page lists the full public configuration surface.
Durations are time.Duration unless stated otherwise. Zero-duration defaults shown
here are applied by constructors; a negative value is not a way to disable expiry.
Invalid static configuration usually panics in New. Register/Shutdown-style
operational APIs return errors as described by their packages.

No constructor opens a database or owns a supplied Redis client. Configure client
TLS, authentication, context/socket/pool timeouts and bounded retries in your
application. Shared transport and codec settings are described once below.

<a id="auth-stateful"></a>

## `auth/stateful`

Use `New[T](redisClient, config)` for the included Redis store, or
`NewWithStore[T](sessionStore, config)` for an application-defined `SessionStore`.
The same configuration applies to both constructors.

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Namespace` | required | Separate the application’s session keys. |
| `AbsoluteTTL` | required, >=1ms | Maximum session age. |
| `IdleTTL` | 0 disables; positive >=1ms | Idle lifetime, moved only by Renew. |
| `OperationTimeout` | 3s | Per-manager operation context budget. |
| `Transport` | secure defaults | See shared transport settings below. |

<a id="auth-jwt"></a>

## `auth/jwt`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `SecretKey` | required, >=32 bytes | High-entropy HS256 key. |
| `Issuer` | required | Expected issuer and refresh key namespace. |
| `Audience` | empty: not required | When set, include and require this audience. |
| `AccessTTL` | 15m; minimum 1s | Access token lifetime. |
| `RefreshTTL` | 7d; minimum 1ms | Maximum individual refresh lifetime. |
| `FamilyTTL` | 0 uses RefreshTTL; positive >=1ms | Absolute refresh family lifetime. |
| `ReplayGrace` | 0 uses 5s | Reuse rejection grace before later-use family revocation. |
| `CookieSecure` | false, for compatibility | Set true for deployed HTTPS cookies. |
| `CookieSameSite` | Lax | Cookie cross-site policy. |
| `CookieDomain` | empty | Cookie domain; normally keep host-only. |
| `CookiePath` | / | Access cookie path. |
| `RefreshTokenPath` | / | Refresh/logout cookie path. |
| `AllowedOrigins` | empty: legacy behavior | Exact origins for checking cookie writes; configure explicitly. |

<a id="auth-stateless"></a>

## `auth/stateless`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Issuer` | required | Issuer for both JWTs. |
| `AccessAudience / RefreshAudience` | required, distinct | Separate access and refresh endpoints. |
| `AccessKey / RefreshKey` | required, distinct, each >=32 bytes | High-entropy HS256 keys. |
| `AccessTTL / RefreshTTL` | required, each >=1s | Credential lifetimes, not a maximum total login age. |
| `Leeway` | 0; allowed 0..1m | Clock skew allowance. |
| `Transport` | secure defaults | Access middleware transport; configure refresh cookies separately. |

<a id="auth-single"></a>

## `auth/single`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `SecretKey` | required, >=32 bytes | High-entropy HS256 key. |
| `Issuer / Audience` | required | Expected claim values. |
| `TTL` | required, >=1s | Access lifetime. |
| `Leeway` | 0; allowed 0..1m | Clock skew allowance. |
| `Transport` | secure defaults | Shared cookie/bearer configuration. |

<a id="auth-transport"></a>

## `auth/transport`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `CookieName` | __Host-session | Credential cookie to set/extract/delete. |
| `Path` | / | Must start with /. |
| `Domain` | empty | Host-only by default; must stay empty for __Host-. |
| `SameSite` | Lax | SameSite=None requires Secure. |
| `InsecureDevelopment` | false | Permit local HTTP; choose an unprefixed cookie name. |
| `AllowedOrigins` | empty | Exact HTTP(S) origins accepted for unsafe cookie-authenticated requests. |

Always call Normalize for manually built transport configs; managers normalize their Transport automatically. Cookies are always HttpOnly. See [cookie rules](security.md#cookies-and-bearer-credentials) for prefix/CSRF requirements.

<a id="cache"></a>

## `cache`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Coalesce` | false | Share concurrent misses within one Loader/process. |
| `Strict` | false | Return lookup/write errors instead of source fallback. |
| `Timeout` | 5s | Source work deadline, independent of individual waiter cancellation when coalescing. |
| `MaxConcurrent` | 128 | Bound on active source loads. |
| `Jitter` | 0; range [0,1) | Randomly shorten successful-value TTL. |
| `NegativeTTL` | 0 disables | Locally cache only ErrNotFound for this duration. |
| `NegativeEntries` | 1024 when negative caching enabled | Bound the local negative cache. |
| `Observer` | nil | Optional nonblocking, concurrency-safe cache.Observer. |

LoaderConfig is used by NewLoader. Direct backends have their own Config. Set/Get TTLs are explicit; zero never means permanent storage.

<a id="cache-memory"></a>

## `cache/memory`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Entries` | required, positive | Maximum retained entry count. |
| `MaxBytes` | 1MiB | Maximum encoded value size. |
| `Namespace` | empty | Local key prefix; namespace+key must fit 4096 bytes. |
| `Observer` | nil | Optional operation/eviction observer. |
| `codec constructor argument` | nil selects JSON[T] | Supply a custom concurrency-safe cache.Codec[T]. |

<a id="cache-redis"></a>

## `cache/redis`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Namespace` | required | Remote keys use namespace:key. |
| `Timeout` | 2s | Operation context budget; configure the client’s socket policy too. |
| `MaxBytes` | 1MiB | Maximum accepted encoded value size. |
| `Observer` | nil | Optional cache.Observer. |
| `client constructor argument` | required | Existing caller-owned redis.UniversalClient. |
| `codec constructor argument` | nil selects JSON[T] | Optional custom codec. |

<a id="cache-tiered"></a>

## `cache/tiered`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `l1 / l2 arguments` | required | Existing cache.Cache[T] backends. |
| `maxL1TTL argument` | required, positive | Maximum local freshness after fills/writes. |
| `observer argument` | nil allowed | Optional population/error observer. |

<a id="authorization"></a>

## `authorization`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Resolver argument` | required | Returns trusted Principal{Roles,Permissions} or an error. |
| `Policy argument to Require` | required | func(context.Context,Principal)(bool,error). |

<a id="httpx"></a>

## `httpx`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `binding struct tags` | application-defined | Use json, uri, form, and Gin binding validation tags. |
| `message/details arguments` | application-defined | Sent to clients: choose safe contents. |

<a id="logger"></a>

## `logger`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Level` | info | debug/info/warn/error; other values fall back to info. |
| `Format` | text | json selects JSON; other values use text. |
| `Output` | stdout | Caller-owned io.Writer; ensure concurrent safety. |
| `MaskSensitives` | false | Enable key-based attribute masking explicitly. |

<a id="middleware"></a>

## `middleware`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `MaxBodySize(bytes)` | required, >0 | Request-body limit enforced when read. |
| `Timeout(duration)` | required, >0 | Cooperative context deadline. |
| `CORSConfig.AllowedOrigins` | required, nonempty | Exact origins or a noncredentialed wildcard. |
| `AllowedMethods / AllowedHeaders` | empty | Permitted preflight methods/headers. |
| `ExposedHeaders` | empty | Response headers browsers may read. |
| `AllowCredentials` | false | Allow credentialed CORS; cannot use wildcard origins. |
| `MaxAge` | 0 | Positive preflight cache duration, in seconds. |

<a id="pagination"></a>

## `pagination`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Query.Page` | 1 when query-bound | Minimum bound value 1; direct values normalize with EnsureDefaults. |
| `Query.Limit` | 10 when query-bound; maximum 100 | Direct EnsureDefaults also clamps to 100. |

<a id="ratelimiter"></a>

## `ratelimiter`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Limit.Rate` | required, >0 | Events per period. |
| `Limit.Burst` | required, >0 | Immediate burst capacity. |
| `Limit.Period` | required, >0 | Quota duration. |
| `Limit.FailureMode` | FailOpen | FailOpen allows on store failure; FailClosed returns 503. |
| `KeyFunc argument` | required | Trusted scope; empty results return 400. |
| `client argument` | required | Caller-owned redis.UniversalClient. |

<a id="health"></a>

## `health`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `New(timeout)` | required, >0 | Overall readiness deadline. |
| `Register(name,Check)` | required | Unique nonempty name and context-aware func(context.Context)error. |

<a id="httpclient"></a>

## `httpclient`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `Timeout` | 10s | Whole request budget, including retries/backoff and body reads. |
| `Retries` | 0; allowed 0..10 | Retries after the initial attempt. |
| `BaseBackoff` | 100ms | Initial exponential-backoff ceiling. |
| `MaxBackoff` | 5s; maximum 1m | Must be >=BaseBackoff. |
| `RetryNonIdempotent` | false | Explicit opt-in; still requires replayable bodies. |
| `RetryStatus` | 429/502/503/504 | Optional func(int)bool policy. |
| `RetryError` | timeout-class net.Errors | Optional func(error)bool policy. |
| `RequestID` | nil | Optional func(context.Context)string header source. |
| `OnAttempt` | nil | Optional func(context.Context,int,time.Duration,error). |
| `client argument` | nil uses cloned standard Transport | A supplied http.Client is copied; transport/jar remain shared. |

<a id="observability"></a>

## `observability`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `New(meter)` | required | Application-supplied metric.Meter. |
| `Middleware(tracer,propagator)` | nil values allowed | Optional trace creation and incoming trace extraction. |
| `Record labels` | application-defined | Keep component/operation bounded and non-sensitive. |

<a id="lifecycle"></a>

## `lifecycle`

| Field or argument | Default / requirement | Controls |
|---|---|---|
| `New(timeout)` | required, >0 | Overall cooperative shutdown budget. |
| `Register(name,cleanup)` | required | Nonempty name and func(context.Context)error. |
| `RegisterHTTP(name,server)` | required | Named nonnil standard HTTP server. |

## Return values and common errors

| API | Success | Common failure |
|---|---|---|
| Stateful Create | (credential, nil) | Marshal/store error, or ErrCollision after bounded collision retries |
| Stateful Verify/Renew | (T, nil) | stateful.ErrInvalid or a store error |
| Single Verify / Stateless VerifyAccess/VerifyRefresh | (T, nil) | transport.ErrInvalid |
| Hybrid Refresh | nil and new cookies | Hybrid refresh sentinel or storage/serialization error |
| Cache Get | (Entry[T], nil) | cache.ErrMiss, ErrCodec, ErrTooLarge, or backend error |
| Loader GetOrLoad | (T, nil) | Source error, ErrBusy, ErrLoaderPanic, or strict-mode cache error |
| httpx.Bind* | (T, true) | (zero, false), with 400 already sent |
| lifecycle.Shutdown | nil | Joined named cleanup errors / context error |

Use errors.Is for sentinels. Hybrid does not use transport.ErrInvalid for its own
JWT/refresh verification errors. Full behavior: [authentication](authentication.md),
[caching](caching.md), and [operations](operations.md).
