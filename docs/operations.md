# Running a Core application

The [README](../README.md) contains everyday examples. This guide covers resource
ownership, HTTP behavior and checks to plan before deployment.

## Redis client ownership

Auth, caching and rate limiting accept an existing redis.UniversalClient. They
neither connect during construction nor close it on your behalf. Configure
ContextTimeoutEnabled=true, bounded dial/read/write/pool timeouts, bounded retries,
TLS/authentication and ACLs in the application. A wrapper timeout does not change
a client configured to ignore context socket deadlines.

Use dedicated namespaces and policies for authentication versus disposable cache
data. Cache eviction is normal; losing auth state makes sessions fail closed.
Persistence rollback/failover can have different effects from losing state, so
review the datastore guarantees your application requires. No cache/session
component provides stronger durability than its Redis deployment.

Stateful namespaces share one Cluster slot; hybrid new refresh families use one
slot per family. Cache operations are single-key. Real multi-node Cluster and
Sentinel failover were not exercised in the recorded validation. Legacy hybrid
refresh migration is standalone-only; see [authentication](authentication.md#hybrid-jwt).

## HTTP responses

httpx success responses use success=true with data; zero/nil data can be omitted
because it uses omitempty. Error responses use success=false and an error object
with code, message and optional details. Envelope types are internal, while
OK/Created/NoContent and Abort helpers are public.

BindJSON/BindURI/BindQuery return (T,bool). A false result means a 400 was already
sent. Return immediately. Validation errors expose field/tag pairs; other parser
errors use a generic detail. AbortInternal sends a fixed client message and logs
the route without the raw error. Application-provided messages/details are sent
as supplied and must not contain infrastructure secrets.

Committed responses are not written a second time by success/abort body helpers;
this does not make continuing an aborted handler safe. Use explicit control flow.
The [httpx package README](../httpx/README.md#error-helpers) lists every Abort status.

## HTTP middleware

SetTrustedProxies should reflect the real proxy boundary. RequestID reuses an
existing response ID, accepts bounded incoming IDs, or generates one. It adds
Gin and logger context metadata. It is correlation, not trusted identity.

MaxBodySize uses http.MaxBytesReader and limits reads, not request admission.
A binder may report 400 rather than 413 for oversized bodies. Timeout sets a
cooperative request context deadline; it does not run a handler in a background
goroutine, interrupt writes or guarantee a 504. Configure standard server
ReadHeaderTimeout, ReadTimeout, WriteTimeout and IdleTimeout too.

CORS validates configured origins, requested preflight methods/headers and
wildcard/credential combinations. Accepted preflights return 204, denied ones
403. A denied ordinary origin receives no allow-origin header, but its handler
can still run: CORS is not an authentication firewall. Use exact credentialed
origins and independent CSRF protection. SecurityHeaders sets common headers;
choose CSP/HSTS based on application content and TLS deployment.

## Pagination

Query-bound Page defaults to 1 and Limit to 10, with maximum limit 100. Direct
construction should use EnsureDefaults before reading fields. Offset normalizes
a copy and saturates at math.MaxInt instead of overflowing. NewResult uses exact
integer page counts, clamps negative totals to zero and returns an empty array
for nil items. It never runs a database query. Deep offsets may still need an
application limit or cursor pagination.

## Rate limiting

The implementation uses redis_rate's GCRA, not a sliding window. Replicas sharing
a key share its quota. Include route/tenant/policy scope in your KeyFunc where
quotas should differ. ByIP uses Gin ClientIP, so configure trusted proxies.

Exceeded quotas return 429 with rounded-up Retry-After seconds. FailOpen, the
legacy default, permits on datastore failure; FailClosed returns 503. Empty keys
return 400. Invalid limits or a nil KeyFunc panic during middleware setup.

Keys are hashed before storage so API-key-based identifiers are not written in
plaintext. Upgrading from the original unhashed layout resets that keyspace once;
plan rollout rate budgets. Dragonfly RESP3 numeric replies are normalized and
validated before redis_rate consumes them. Core never closes the Redis client.

## Health checks

Liveness returns 200 without external work. Readiness returns 200 only when all
registered checks pass, otherwise 503, without public names/errors. Ready(ctx)
returns named booleans for private diagnostics. Duplicate names, nil checks and
nonpositive registry timeouts are rejected during setup.

Readiness checks share an overall deadline, with at most 32 active calls across
requests. A check that ignores context keeps its slot; later checks fail readiness
rather than spawning unlimited replacement workers. Make checks cheap and
context-aware, and restrict excessive public probing. Dependency outages usually
belong in readiness rather than liveness.

## Outbound HTTP

New copies a supplied http.Client but shares its transport/jar. Nil uses a cloned
standard Transport. Customize connection pools on a caller-owned transport.
CloseIdleConnections explicitly affects the shared pool; only call when ownership
permits. Core does not install a tracing SDK or replace the standard transport.

Timeout covers attempts, backoff and response-body reads; earlier caller/client
deadlines still apply. Always close the final body to release the budget and
connection. Intermediate bodies are closed without indefinite draining, which
can sacrifice reuse. Limit response reads according to expected data size.

Retries are bounded to 0..10 and default to zero. Standard idempotent methods may
retry; request bodies require GetBody for replay. Non-idempotent retries need an
explicit opt-in and safe upstream semantics such as application deduplication.
Default retry statuses are 429/502/503/504; default network policy matches timeout
net.Errors. Custom callbacks can override those policies.

Backoff is exponential with full jitter and a finite MaxBackoff. Retry-After
seconds/dates provide a minimum delay. If that delay exceeds MaxBackoff, return
the response for the application to handle rather than retrying early. Context
cancellation interrupts backoff. RequestID and OnAttempt integrate correlation
and an existing logger/metrics system; avoid sensitive error/URL logging. Supply
an instrumented standard RoundTripper for outbound tracing when needed.

## Observability

Applications own OTel providers/exporters and their shutdown. Core creates API
instruments only, without changing global providers. No-op providers work, and
omitting middleware/observers disables instrumentation.

| Metric | Unit / attributes |
|---|---|
| core.operation.duration | seconds; component, operation |
| core.operation.calls | counter; same attributes |
| core.operation.errors | counter; same attributes |

Cache observations classify ok/miss/error; misses are not backend errors. HTTP
spans/metrics use Gin route templates and a constant unmatched label. Record
accepts your own bounded dependency operation names. Never use raw keys, URLs,
IDs, credentials or personal data as labels. Tracer/propagator can be omitted
independently. Core's slog logging remains a separate existing facility.

## Shutdown

Register dependencies first and HTTP last so reverse cleanup drains requests
before closing clients. Use signal.NotifyContext or your own signal handling,
then call Shutdown with a fresh context after the signal cancels the running one.
See the [runnable server](../examples/server/main.go).

Hooks run sequentially once, even with repeated/concurrent Shutdown calls.
Failures are joined with names; later hooks still run. Panics are contained
without exposing their values. Registration after shutdown starts returns an
error. Concurrent callers can cancel their own wait.

The deadline is cooperative: cleanup functions must honor context. They are not
abandoned in background goroutines and cannot be forcibly terminated. Later
hooks receive the expired context so they can do bounded emergency cleanup.
Avoid reentrant Shutdown calls and cyclic resource waits.

## Testing your integration

Run from the repository root:

```sh
go test ./...
go test -race ./...
go vet ./...
```

These compile documentation snippets, execute the memory example, and validate
local Markdown links. Datastore tests skip without explicit service opt-in.
Use disposable instances for the integration scripts:

```sh
scripts/integration.sh redis:7.4.2-alpine
scripts/integration.sh docker.dragonflydb.io/dragonflydb/dragonfly:v1.27.1
```

Scripts create/remove loopback containers, run the full race suite, restart the
service and rerun datastore tests. CORE_INTEGRATION=1 and CORE_REDIS_ADDR can
explicitly select another disposable loopback instance. Do not use production
stores. No test relies on FLUSHDB or unrestricted KEYS.

Recorded Redis restart and injected acknowledgment-loss cases do not model every
partition, replica rollback or in-flight crash. The [implementation validation
report](reports/VALIDATION.md) records what actually ran; the
[benchmark guide](benchmarks.md) explains measurements separately.
