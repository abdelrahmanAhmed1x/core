# Validation report — 2026-10-20

This report records executed checks, not a general production-readiness
certification. See ../authentication.md and ../caching.md for exact guarantees and restrictions.

## Implementation and compatibility

New packages: auth/stateful, auth/single, auth/stateless, auth/transport,
auth/internal/token, authorization, cache (contracts/loader/codec), cache/memory,
cache/redis, cache/tiered, health, httpclient, observability and lifecycle.
Existing auth/jwt, httpx, logger, middleware, pagination and ratelimiter were
hardened without removing exported Go signatures or import paths.

Behavior changes to review during rollout:

- Hybrid new refresh credentials/family keys require a one-way format migration.
  Standalone legacy tokens can migrate; legacy Cluster sessions must log in again.
  Family lifetime is now absolute, not extended indefinitely by refresh. Legacy
  access JWTs without type claims remain accepted; optional Audience is opt-in.
- Hybrid cookie security keeps its old configuration defaults for compatibility;
  configure CookieSecure and AllowedOrigins explicitly. Ambiguous credential
  transports are rejected, including in optional middleware; invalid single
  credentials retain legacy optional-as-guest behavior.
- Redis rate identifiers are hashed, resetting the previous counter keyspace
  once on upgrade. Empty identifiers are rejected. Dragonfly RESP3 count replies
  are normalized/validated to avoid the existing redis_rate panic.
- Pagination direct construction now clamps limits and saturates overflow.
  Binding errors and access/internal logs expose fewer potentially sensitive
  details. Child logger redaction now matches root logger redaction.
- Added Config fields are additive for keyed literals; unkeyed external struct
  literals are inherently sensitive to field additions and need conversion to
  keyed literals. No exported interfaces were expanded.

## Executed checks

Combined statement coverage on Go 1.26.9 with Redis is **72.7%**, including
example commands and integration helpers in the denominator. This is not full
branch coverage or a security proof.

The repository originally had no tests. The implementation adds table-driven
unit/security tests, service integration tests, failure injection, concurrency
tests, fuzz targets and benchmarks. Cache adapter tests live in cache's external
test package and exercise all three adapters; their individual directories do
not contain duplicated test files.

Executed locally:

- go fmt ./..., go vet ./..., git diff --check.
- Full tests on declared minimum Go 1.26.5 with real Redis.
- Full tests and race detector on installed Go 1.27.1 with real Redis and Dragonfly.
- Full race/combined coverage on patched Go 1.26.9 with Redis.
- scripts/integration.sh for Redis 7.4.2-alpine and Dragonfly v1.27.1, including
  disposable-container startup, full race suite, actual process restart, and
  auth/cache/rate-limit suites after restart. Scripts re-read ephemeral ports
  because this Docker environment can reassign them on restart.
- Runnable examples/auth and examples/cache (without optional Redis mutation).
- 3-second fuzz budgets with two workers: JWT parsing 26,471 executions,
  credential extraction 16,119, JSON decoding 60,113; all passed. These short
  runs are smoke checks, not exhaustive fuzz campaigns.
- go test -run '^$' -bench . -benchmem -benchtime=300ms ./... with real loopback
  Redis. Complete measured output is in benchmarks-go127-redis.txt.

Test cases cover forged/none/wrong JWT algorithms, issuer/audience/type confusion,
expiry/missing claims, session loss/revocation/generation isolation, exact large
integer payloads, idle renewal/absolute bounds, refresh races/reuse/family loss,
legacy refresh migration, pre-commit injected failures, successful-commit/lost-
acknowledgment injection, malformed storage, conflicting transports/CSRF/cookies,
optional-auth failures, authorization denial/error boundaries, cache errors,
expiry/capacity/copy isolation, same/different-key loads, cancellation/concurrency
bounds, stale-fill suppression, bounded lost-invalidation staleness, HTTP retry
bodies/Retry-After/cancellation, health timeouts, reverse/idempotent shutdown,
logging redaction and existing-package boundaries.

The restart script verifies operations before and after real service restarts;
it does not claim exhaustive network partition, replica failover, persistence,
or in-flight process-crash modeling. Failure hooks separately inject the
important before/after-commit refresh acknowledgment cases.

## Dependency vulnerability scanning

Initial govulncheck v1.1.4 could not analyze the current Go AST and panicked.
The pinned scanner was updated to v1.9.0. Scanning the original dependencies and
installed Go 1.27.1 found reachable advisories. Patched the existing dependencies:
quic-go 0.59.0 → 0.59.1 and x/net 0.51.0 → 0.60.0, with compatible x/crypto,
x/sync, x/sys and x/text upgrades pinned by go.mod/go.sum.

The recorded scan on **Go 1.26.9** reported **zero reachable vulnerabilities and zero
advisories in imported packages**. It reports one module-only advisory,
GO-2026-5932, for the unmaintained x/crypto/openpgp package, which Core does not
import or call; no fixed version exists. Saved output: govulncheck.txt.

Go 1.26.5 remains the source compatibility minimum, not a recommended deployment
patch level. Deploy using Go 1.26.10 or 1.27.3 or a later patched release, and keep
scanning. CI tests both 1.26.5 and 1.26.10, and runs the vulnerability gate and
integration tests on 1.26.10. The earlier recorded scan predates GO-2026-6809,
which requires this toolchain update. Official advisory
references: [Go HTTP fixes](https://pkg.go.dev/vuln/GO-2026-6613),
[x/net HTTP/2 fixes](https://pkg.go.dev/vuln/GO-2026-6617),
[quic-go fix](https://pkg.go.dev/vuln/GO-2026-5676),
[x509 certificate fix](https://pkg.go.dev/vuln/GO-2026-6809).

## Benchmark environment and interpretation

Linux 7.0.0-38-generic, amd64, Intel Core i7-10700K @ 3.80GHz, 16 logical CPUs,
Go 1.27.1, default GOMAXPROCS=16. Redis 7.4.2-alpine runs in Docker on loopback
16379 with no production TLS/replication/failover latency. Dragonfly protocol and
race integration were measured for correctness; no Dragonfly performance claim
is made. Background validation/compilation shared this workstation during the
recorded run; these are illustrative single-run measurements, not isolated
capacity planning figures or statistically established regressions.

| Operation | ns/op | B/op | allocs/op | Approx operations/sec |
|---|---:|---:|---:|---:|
| JWT verify | 9,979 | 2,727 | 49 | 100,210 |
| Stateful create, networked | 79,819 | 2,131 | 38 | 12,528 |
| Stateful verify, networked | 166,941 | 2,656 | 42 | 5,990 |
| Hybrid refresh incl. HTTP harness | 282,352 | 16,387 | 162 | 3,542 |
| Memory hit, JSON int | 438.1 | 8 | 1 | 2,282,584 |
| Memory miss | 100.7 | 0 | 0 | 9,930,487 |
| Redis hit | 110,764 | 801 | 22 | 9,028 |
| Redis miss | 148,253 | 1,000 | 39 | 6,745 |
| Tiered warmed L1 hit | 519.8 | 8 | 1 | 1,923,817 |
| Memory set | 302.8 | 16 | 2 | 3,302,510 |
| Redis set | 95,480 | 720 | 16 | 10,473 |
| Redis set+delete pair | 225,860 | 1,346 | 29 | 4,427 pairs |
| JSON map encode+decode | 1,108 | 441 | 9 | 902,527 pairs |
| Parallel coalesced misses | 3,656 | 433 | 7 | 273,523 |
| Parallel different-key loads | 7,423 | 845 | 18 | 134,716 |

Throughput is 1e9/ns-per-operation, rounded, not a separate load test. Warmed
tiered benchmarks mostly measure L1. MemoryDelete measures deletion of an absent
key after the initial removal; Redis set+delete includes both operations.
Parallel coalesced benchmark uses a deliberately always-missing backend; it is
not a database throughput measurement. Authorization/header middleware figures
in the raw file include Gin/httptest allocations and are not pure predicate or
header-write costs. Serialization and network latency materially affect results.

## Remaining operational risks and unvalidated scenarios

Redis Cluster routing is designed with explicit same-slot auth operations and
single-key cache operations, but a real multi-node Cluster/Sentinel failover
suite was not executed. Pub/Sub and distributed locks are deliberately absent;
L1 has bounded staleness rather than coherence. DB/cache cross-process ordering
must use application versioning/coordination. Sessions require authoritative,
properly secured Redis state; persistence rollback, eviction and failed replicas
have application-specific availability/revocation consequences. Redis Lua is
atomic but not a rollback transaction for arbitrary runtime errors/OOM.

No fuzz duration, coverage percentage or scanner result proves absence of
security defects. Refresh replay grace cannot distinguish delayed legitimate
retries from theft. Absolute family TTL and Redis persistence/ACL policies need
application review. Caller callbacks/checks/hooks must cooperate with context;
Go cannot terminate stuck goroutines. Cache/session payload sizing, key rotation,
password/MFA verification, login CSRF, resource authorization, trusted proxy
configuration and rollout rate budgets remain application responsibilities.
