# Baseline audit (2026-10-20)

Module: github.com/abdelrahmanAhmed1x/core; minimum Go 1.26.5.
Installed toolchain: Go 1.27.1 linux/amd64. Baseline `go test ./...` passed
with no tests in any of the six existing packages. All source and README
examples were inspected before editing. No existing worktree changes.

Compatibility surface: jwt.Authenticator[T], jwt.Config, jwt.New and exported
errors; httpx response/binding/abort helpers; logger.Logger, Config, context
helpers and middleware; middleware constructors and CORSConfig; pagination.Query,
Meta, Result and NewResult; ratelimiter.New, Limit, KeyFunc, FailureMode, ByIP.
Existing signatures and import paths are retained.

Findings: hybrid GETDEL consumed the credential before signing/persisting its
replacement; no replay family; cookie-only hybrid adapters; no audience/type
claim policy; logger.With bypassed masking, LogValuer was unresolved before
masking; wildcard CORS never matched; pagination overflow and floating-point
page count; binding exposed parser errors; raw error/path logging could contain
secrets. Timeout middleware sets a cooperative deadline, not a response timeout.
Rate limiting uses redis_rate's GCRA (not a sliding-window implementation).

Original direct dependencies: gin 1.12.0, validator 10.30.1, redis_rate 10.0.1,
golang-jwt 5.3.1, go-redis 9.22.0. Dependency versions remain pinned in go.mod.
