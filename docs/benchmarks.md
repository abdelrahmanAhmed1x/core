# Benchmarks and validation evidence

These are recorded implementation measurements, not production capacity promises.
Use your application data, codecs, network and datastore topology when measuring.

## Reproduce

```sh
go test -run '^$' -bench . -benchmem -benchtime=300ms ./...
```

Memory/JWT benchmarks run without services. For networked benchmarks, start a
disposable Redis or DragonflyDB instance and opt in explicitly:

```sh
CORE_INTEGRATION=1 CORE_REDIS_ADDR=127.0.0.1:16379 \
  go test -run '^$' -bench . -benchmem -benchtime=300ms ./...
```

The address must be a loopback IP. Benchmarks can create keys, so never use an
application production instance. See [integration operations](operations.md#testing-your-integration)
for disposable containers.

## Recorded environment

Go 1.27.1, Linux 7.0.0-38-generic amd64, Intel Core i7-10700K @3.80GHz,
16 logical CPUs, default GOMAXPROCS=16. Redis 7.4.2-alpine ran in Docker on loopback
16379. The workstation also ran validation/compilation during this single recorded
run, so scheduler/resource contention affects the timings.

| Operation | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| JWT verification | 9,979 | 2,727 | 49 |
| Stateful creation, networked | 79,819 | 2,131 | 38 |
| Stateful verification, networked | 166,941 | 2,656 | 42 |
| Hybrid refresh including HTTP harness | 282,352 | 16,387 | 162 |
| Memory hit, JSON int | 438.1 | 8 | 1 |
| Memory miss | 100.7 | 0 | 0 |
| Memory set | 302.8 | 16 | 2 |
| Redis hit | 110,764 | 801 | 22 |
| Redis miss | 148,253 | 1,000 | 39 |
| Warm tiered L1 hit | 519.8 | 8 | 1 |
| Redis set | 95,480 | 720 | 16 |
| Redis set + delete pair | 225,860 | 1,346 | 29 |
| JSON map encode + decode | 1,108 | 441 | 9 |
| Parallel coalesced misses | 3,656 | 433 | 7 |
| Parallel different-key loads | 7,423 | 845 | 18 |

[Raw output](reports/benchmarks-go127-redis.txt) also includes parallel reads,
header middleware and authorization measurements. Approximate operations/second
can be computed as 1e9 divided by ns/op; that is not a separate load test.

## Interpret the numbers

Warm tiered reads mostly measure L1. MemoryDelete mostly measures absent-key
deletion; Redis set+delete includes both calls. The coalesced benchmark uses an
always-missing fake backend, not a database workload. Different-key benchmarks
include key creation and scheduler costs. Authorization/header benchmarks include
Gin/httptest allocations, rather than isolating the predicate/header operation.

Loopback Docker figures omit production TLS/replication/failover latency and do
not establish statistical regressions from one run. Dragonfly compatibility/race
tests passed, but these numbers are Redis measurements, not a Dragonfly performance
claim. The Go version describes this historical run, not a deployment recommendation;
use a maintained patched toolchain for new builds.

## Other evidence

- [Implementation audit](reports/AUDIT.md): original API/dependency review.
- [Implementation validation](reports/VALIDATION.md): unit, race, integration, failure and fuzz scope.
- [Redis test output](reports/test-results-go1269-redis.txt) and [Dragonfly output](reports/test-results-go1269-dragonfly.txt).
- [Coverage](reports/coverage.txt) and [vulnerability scan](reports/govulncheck.txt).

These artifacts predate the documentation rewrite. The documentation checks added
here compile snippets and check local links; they do not claim a new performance
or datastore validation run. No coverage/scanner result proves absence of defects.
