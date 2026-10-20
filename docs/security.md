# Security notes

Core supplies authentication and HTTP helpers. Applications still own identity
verification, key management, authorization rules and their deployment boundary.
The [README](../README.md) calls out the limits needed to choose a package; this
page collects the details for application review.

## Credentials and keys

Only issue a credential after your application verifies identity (for example,
password and MFA). Core does not provide a password database, login verifier,
OAuth server or user model. Rate-limit login/refresh and protect credential delivery.

Stateful credentials and opaque hybrid parts use crypto/rand; Redis stores their
hashes rather than raw usable credentials. JWTs use the library's fixed HS256
policy. Supply high-entropy keys of at least 32 bytes from a secret manager;
stateless access/refresh keys and audiences must be distinct. Do not use a human
password as a signing key. Key rotation is application-managed; removing a key
is coarse revocation, not immediate per-session control.

JWT payloads are readable, not encrypted. Stored session payloads are JSON, not
encrypted by Core. Keep both small and omit passwords, API keys, signing material
and unnecessary personal data. Protect Redis with appropriate ACLs, TLS and network
isolation. Choose persistence/replication/eviction policies suitable for auth state.

## Revocation

Stateful revoke rejects later verification after it commits, but cannot stop an
already-authorized handler. There is no stale local session cache.
Hybrid logout/replay revokes refresh state; access JWTs continue until expiry.
Single/stateless JWTs cannot be individually revoked before expiry, and stateless
refresh does not invalidate earlier credentials. A fully stateless pair is not
single-use rotation or replay protection. See [the auth guide](authentication.md)
for the exact family grace, upgrade and delivery-failure behavior.

## Cookies and bearer credentials

Manually built transport configs must pass through Normalize. Defaults are a
Secure, HttpOnly, SameSite=Lax __Host-session cookie, Path=/ and no Domain.
__Host- names require Secure, Path=/ and no Domain; __Secure- requires Secure.
For local HTTP development explicitly choose an unprefixed name and
InsecureDevelopment=true. SameSite=None requires Secure.

Extract rejects duplicate Authorization headers, duplicate named cookies and
unequal bearer/cookie credentials. Equal credentials still require cookie Origin
checks. A state-changing cookie request needs an exact allowed HTTP(S) Origin;
missing, opaque, repeated or untrusted origins are rejected. Never use a wildcard
trusted origin or put state changes in GET handlers.

SetCookie with a nonpositive TTL deletes with matching name/path/domain settings
and an expired timestamp. This is client-side deletion: revoke stored sessions
separately, and do not claim a deleted JWT is invalid everywhere.

For cross-origin browser requests configure exact CORS origins, credentials and
the appropriate Secure/SameSite policy. CORS alone is not CSRF protection. Login
can be a CSRF target before any cookie exists: add application login CSRF/Origin
checks independently. Bearer-only native requests do not need ambient-cookie CSRF
checks, but still need TLS and safe credential storage.

Hybrid keeps CookieSecure=false and empty AllowedOrigins as compatibility defaults;
set both explicitly for deployed cookies. Extraction errors on hybrid Refresh
map to ErrMissingRefreshToken, not to an exposed transport error. The configured
check still rejects the request. Do not expose internal errors to the client.

## Authorization

Resolve roles/permissions only from a verified principal or trusted service.
Return authorization.ErrUnauthenticated when no principal exists, including when
using optional authentication. Empty role/permission requirements deny; policy
false is 403, and resolver/policy infrastructure failures are opaque 503.

Permissions in JWTs can go stale. Query a current database/service in Resolver
when immediate permission changes matter. Applications implement ownership,
tenant membership and business transitions. A successful login is not permission
to access every resource.

## Logging

MaskSensitives is opt-in. Masking matches known attribute names case-insensitively,
including nested groups, With attributes, sensitive group scopes and LogValuer
results. It does not scan arbitrary messages, unknown keys, error strings or
contents of arbitrary maps/structs. Never put credentials in those places.
Direct Slog calls do not automatically extract Core's request context attributes.

Request IDs are client-controlled correlation metadata, not identity. Access logs
use route templates and error counts instead of raw paths/errors/IPs. Applications
still choose writer safety, privacy policy, retention and their own logging fields.
OTel labels must not include user/session IDs, raw cache keys, credentials or URLs.

## Application data caches

General cache source-fallback defaults are inappropriate as a security authority.
Do not use an L1 cache to decide session revocation or permission freshness.
Tiered values can be stale across processes until their local TTL. Source callbacks
for a key must represent the same tenant/user scope; key collisions between scopes
can disclose data. Database invariants need source-of-truth transactions, not
cache-aside ordering. See [cache guarantees](caching.md).

## HTTP boundary and validation

Configure Gin trusted proxies for the actual ingress; otherwise ClientIP-based
quotas can use untrusted forwarding data. SecurityHeaders is a starting set, not
an application-specific CSP/HSTS policy. Body limits are enforced on reads;
server timeouts and context-aware dependencies are separate requirements.

httpx hides generic parser details and returns validator field/tag pairs instead
of rejected values. Its application-supplied messages/details remain your
responsibility. Aborting/binding still requires returning from the handler.

## Validation scope

Tests, race detection, short fuzz runs and vulnerability scanning provide evidence,
not proof against every threat. The [historical validation report](reports/VALIDATION.md)
lists actual versions, executed scenarios and untested Cluster/Sentinel failover.
Use a maintained patched Go toolchain and rescan dependencies for your release.
