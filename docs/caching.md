# Caching guide

Use [the README](../README.md#caching) for constructors and everyday calls.
All backends implement Get(ctx,key), Set(ctx,key,value,ttl), and Delete(ctx,key).
Get returns Entry[T]{Value,ExpiresAt}; a miss is cache.ErrMiss.

## Choose a backend

| Mode | Contents shared between processes? | Extra local staleness? |
|---|---|---|
| Memory | No | Each process keeps its own values |
| Redis/DragonflyDB | Yes | No L1 copy |
| Tiered | L2 shared, L1 private | Yes, up to local TTL for existing copies |

Choose Redis-only when avoiding private L1 copies matters more than the extra
network read. Choose tiered mode for hot data where a short stale window is
acceptable. None of these caches makes database updates transactional with
cache reads/writes. Do not use cached application data to enforce session
revocation, payment, inventory or permission correctness.

## Memory cache

The memory adapter uses HashiCorp golang-lru/v2 with synchronous admission. A
successful Set is visible to later reads unless another operation evicts/deletes
it. Unlike an asynchronous admission cache, there is no write queue to wait for.
There are no workers and no Close method.

Entries bounds the number retained, MaxBytes the encoded value size. This is not
exact heap accounting: keys, metadata, caller values and transient serialization
allocations are additional. Keys including namespace are bounded to 4096 bytes.
Expired data immediately misses, but its allocation may remain until replacement,
eviction or Purge. Idle memory can therefore remain at the configured bound.

JSON copies values on write/read, avoiding shared mutable map/slice references.
Custom codecs must preserve ownership and be concurrency-safe.

## Redis and DragonflyDB

The adapter uses a caller-owned redis.UniversalClient and stores keys as
namespace:key. Choose versioned namespaces that do not collide with other apps.
Configure network/pool timeouts, bounded retries, TLS/ACLs and
ContextTimeoutEnabled=true on that client; wrapper context deadlines do not
silently change client socket options. The cache never closes the client.

A single-key Lua read obtains value and remaining TTL together. This needs
scripting permissions and follows single-key Cluster routing. Expiration is
estimated from request start time, so network latency can shorten the returned
freshness rather than extending it. A script cache miss may add a setup roundtrip.
The commands are Redis-compatible; Dragonfly protocol tests are recorded in
[the validation report](reports/VALIDATION.md).

Set requires TTL >=1ms; it never creates a permanent entry. Missing/expired data
and externally inserted permanent entries return ErrMiss. Remote failures remain
errors. An error after SET may mean the reply was lost after a successful write;
it is not proof the value was not stored.

MaxBytes limits values accepted for read/write. An oversized external value has
already arrived over the network before it can be rejected: use Redis ACLs and
resource limits as well.

## Tiered cache

Reads try L1 then L2. An L2 hit may populate L1 for the smaller of remaining L2
freshness and maxL1TTL. This does not intentionally extend the observed L2 expiry.
Set writes L2 before L1; Delete attempts both and joins errors.

An L1 read/population error is sent to an installed observer and a successful L2
result stays usable. An L2 failure after an L1 miss is returned. Existing L1 hits
can remain usable until expiry. Set may succeed remotely and fail locally: there
is no all-or-nothing rollback.

A write/delete in one instance does not clear copies in another instance. This
implementation uses bounded L1 TTL, not Pub/Sub. It does not claim durable
invalidation or strong coherence. For stronger ordering use Redis-only reads,
versioned keys/namespaces or explicit application coordination.

## Loading missing values

GetOrLoad checks the cache first, then your function, then stores its successful
result. Coalesce=true enables singleflight: concurrent same-key requests within
that Loader/process can share a source call. Different keys can load concurrently
up to MaxConcurrent; extra loads return ErrBusy rather than queuing indefinitely.
Cache hits do not enter singleflight.

A waiting caller can cancel independently. Shared work retains the first caller's
context values but uses the configured Timeout, detached from that caller's
cancellation/deadline. Source functions must honor the work context; Go cannot
kill a stuck callback. With coalescing disabled, work inherits caller cancellation.
Callbacks for a key must describe the same data/tenant/authorization scope.
Singleflight does not prevent stampedes between application instances.

### Failure policy

| Condition | Default | Strict=true |
|---|---|---|
| Miss | Load source | Load source |
| Backend lookup/decode error | Notify observer, load source | Return cache error |
| Source error | Return it, do not cache | Same |
| Cache write error | Notify observer, keep successful source value | Return write error |
| Source panic | ErrLoaderPanic, without panic value | Same |
| Load limit reached | ErrBusy | Same |

Observer=nil intentionally disables reporting. Install an observer for visibility
when using default source fallback. Cancellation and failed source calls do not
become successful cached zero values.

Jitter is a fraction [0,1) that only shortens positive TTLs. Negative caching is
off by default. Set NegativeTTL>0 to locally cache only errors matching
cache.ErrNotFound, up to NegativeEntries. It does not cache arbitrary errors,
share negative entries remotely or persist them through restart. Store/Invalidate
clear that Loader's negative entry; keep negative TTL short for newly created data.

### Updating data

Use Store to update a key through the same Loader and Invalidate to remove it.
These operations suppress older in-flight fills from overwriting a newer local
write. An already-loading request can still return its earlier source snapshot.
Invalidation permits later requests to start a fresh load.

The implementation uses fixed lock/generation stripes rather than unbounded
per-key lock state. Unrelated colliding keys may conservatively skip a fill.
Direct backend writes and other Loader instances/processes bypass this ordering.
Database/source writes still need correct application ordering. Use versioned
values/keys when cross-instance stale-fill races are unacceptable.

## Codecs, TTL and errors

Nil codec uses encoding/json. Codec[T] supplies Encode(T)([]byte,error) and
Decode([]byte)(T,error). Unsupported values return serialization errors; malformed
stored data is never a successful zero value. Stored null is a hit for compatible
payload types. Prefer concrete types for integer accuracy; use compatible
schema changes or version namespaces.

Custom encoders must return buffers safe to retain; do not reuse mutable storage
behind the cache. MaxBytes applies to encoded size, not arbitrary codec work.

| Error | Meaning |
|---|---|
| ErrMiss | Missing or expired entry |
| ErrTTL | Invalid lifetime: nonpositive, or Redis TTL below 1ms |
| ErrCodec | Encoding/decoding failure |
| ErrTooLarge | Encoded value too large, or an oversized loader key |
| ErrNotFound | Source explicitly says the resource does not exist |
| ErrBusy | Loader concurrency limit reached |
| ErrLoaderPanic | Source callback panicked |

Memory supports any positive TTL; Redis needs at least 1ms. Invalid TTL never
silently becomes permanent storage. Deletes are idempotent. Canceled contexts
return errors according to the backend/client's cancellation support.

## Observability

Config.Observer is optional on memory, Redis, tiered and loaders. It receives
operation, duration and error, never keys/values. observability.Instruments adapts
this to OTel counters/histograms. Built-in labels cover L1/L2 outcomes, loader calls,
write/population errors, codecs, capacity eviction and shared flights. Lazy expiry
is not capacity eviction. Custom observers must be nonblocking/concurrency-safe.
See [benchmarks](benchmarks.md) for measured costs and limitations.
