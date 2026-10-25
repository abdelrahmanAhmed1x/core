# Authentication guide

Start with the [README comparison](../README.md#authentication) to choose a strategy.
This guide covers browser/mobile integration and behavior that matters during
refresh, revocation and upgrades. For field defaults see [configuration](configuration.md).

## Shared Gin routes

Every manager has Middleware(), OptionalMiddleware(), and Payload(c).
Use Middleware on routes that require login. Payload returns your generic type
and a boolean. All four also populate `transport.Payload[T](c)`. Hybrid keeps its
original context key and MustPayload method for compatibility; MustPayload panics
when no authenticated user is present.

Required middleware rejects missing/invalid credentials with 401. Stateful store
errors return an opaque 503. New strategies' optional middleware permits only a
missing credential as a guest; supplied invalid credentials still return 401.
Hybrid preserves its older optional behavior: one invalid credential becomes a
guest, but conflicting transports or failed configured Origin checks return 401.
Always require authentication or a trusted-principal policy before protected work.

## Stateful sessions

Create(ctx,userID,payload) returns an opaque credential. Verify(ctx,credential)
looks it up; Renew extends idle expiry without moving absolute expiry. Revoke
invalidates one session, and RevokeAll invalidates the user's earlier sessions.

For browsers, call Login in your existing handler **after** identity verification.
It stores the session before setting a cookie. In a different handler call Logout.
Configure Transport.AllowedOrigins for cookie-authenticated writes. Both adapters
return errors; report a storage failure safely rather than claiming revocation
succeeded. Logout clears the cookie after attempted revocation, but an extraction
or CSRF failure returns before revocation/deletion.

For native clients, send Create's returned credential in the authenticated login
response and use Authorization: Bearer on later requests. Call Revoke for logout.
Both transports use the same store. Do not log the credential.

Verification does not automatically renew idle expiry. If your application wants
sliding activity, call Renew explicitly at the chosen interval. IdleTTL=0 disables
idle expiry. Stores enforce absolute/idle expiry independently of physical cleanup.
The included Redis store uses Redis TIME as well as TTL.
Malformed, expired, revoked, missing or corrupted sessions match stateful.ErrInvalid
(the same sentinel as transport.ErrInvalid). Storage errors remain distinct errors.

### Custom session storage

`Manager[T]` depends on `stateful.SessionStore`. Use the existing `New[T](client,
config)` for Redis/DragonflyDB, or inject your application's implementation:

```go
// Your repository implements stateful.SessionStore.
auth := stateful.NewWithStore[User](sessionRepository, stateful.Config{
    Namespace: "shop:sessions", AbsoluteTTL: 24 * time.Hour,
})
```

`stateful.NewRedisStore(client)` exposes the included implementation directly.
The caller owns store/client initialization and shutdown. There is no PostgreSQL
implementation or schema requirement; an application can implement the interface
using its own repository and tables.

The manager generates credentials, hashes credentials and user IDs, encodes the
generic payload as JSON, and applies operation deadlines. The store receives a
`CreateRequest` containing the namespace, hashes, JSON bytes, and expiry durations.
It returns JSON bytes for Verify/Renew; it does not need to know the payload type.

Implement the full session operations atomically, including validation and idle
renewal. Create must reject collisions with `ErrCollision` without overwriting.
Verify/Renew must return `ErrInvalid` for missing, expired, revoked, or corrupt
sessions; preserve infrastructure errors so middleware can return 503. Revoke is
idempotent. Namespace isolation, context cancellation, concurrent safety, and an
authoritative clock are part of the contract. Never extend absolute expiry or
resurrect a session through renewal. Managers sharing a namespace must use the
same AbsoluteTTL so revocation-state retention covers their sessions. See
[`SessionStore`](../auth/stateful/store.go) for the complete method contract.

### Revocation and concurrent creation

Revocation applies to verifications that happen after it commits. It cannot
cancel an action that already authenticated. Create and RevokeAll have a defined
order: sessions created before revoke-all are invalidated; sessions created after
it remain valid. There is no promise that a concurrent login is rejected.

In the included Redis store, each user has a random, expiring generation. Session
creation extends its retention as needed, while sessions keep their original absolute expiry. There is no session
index to enumerate or leak. Missing generation state fails closed.

Tokens use 32 crypto/rand bytes and canonical URL-safe base64. Stores see hashed
verifiers, not the raw credential. In Redis, the payload is JSON inside a string
so Redis metadata processing does not round large integer fields. Payloads are not encrypted;
keep them small and free of secrets.

With RedisStore, all keys in one namespace share a Redis Cluster slot so operations
involving a session and generation can be atomic. This limits distribution within a namespace;
partition namespaces where appropriate. Session checks never use the application
L1 cache. Redis persistence, eviction and failover decisions remain part of the
authentication deployment, not automatic guarantees from this manager.

## Single JWT

Issue(payload) signs one access JWT; Verify(raw) checks it and returns T.
There is no refresh or server-side Logout. Cookie deletion removes the browser's
copy, not any other copy. Use short lifetimes when credentials can be stolen or
permissions can change before expiry.

The algorithm is fixed to HS256. A high-entropy key of at least 32 bytes, issuer,
audience and TTL >=1s are required. Verification checks signature, algorithm,
issuer, audience, expiry, issued-at, token ID, payload presence and access type.
Issued-at cannot be in the future outside configured leeway, and expiry must
follow issued-at. Leeway is 0..1 minute. Issuance and verification reject tokens
larger than 16KiB; browser cookie limits are smaller in practice.

Invalid credentials match transport.ErrInvalid. Issuance can fail when payload
serialization fails. JWT contents are readable: do not include private secrets.

## Stateless pair

Issue returns Pair{Access,Refresh}. VerifyAccess and VerifyRefresh check the
appropriate type/key/audience. Refresh accepts a valid refresh JWT and issues a
new pair using the same payload; it does not reload the user or permissions.
Use separate keys and audiences as required by Config.

Every previous valid access **and refresh** JWT stays usable until its expiry.
There is no single-use rotation, family tracking, reuse detection or individual
revocation. The package has no Logout. Deleting credentials is client-side only.
Repeated refresh can continue issuing new lifetimes; there is no absolute maximum
login age. Choose a stored strategy if those properties are required.

This is not an OAuth authorization server. In particular, it does not supply
OAuth public-client refresh reuse detection or sender-constrained credentials.
Use a suitable authorization server for those protocols. Core's invalid-credential
sentinel here is transport.ErrInvalid, as with single JWT.

## Hybrid JWT

Hybrid retains `New[T](redis.UniversalClient,Config) Authenticator[T]` and its
Gin-based Login/Refresh/Logout API. It has no public standalone Verify method;
use Middleware to verify access credentials. The resulting typed user is available
through Payload or MustPayload.

Login persists refresh state before sending the access_token and refresh_token
cookies. Access verification is independent of Redis. Logout marks the refresh
family revoked and clears both cookies, even if revocation returned a storage
error. Existing access JWTs continue working until their own expiration.

### Rotation and replay policy

New refresh credentials contain random family and secret parts, each with 256
random bits. Redis stores their hashes. Family keys share a slot. Refresh fetches
the payload and prepares a new access token/refresh secret before changing storage.
The server-side transition checks current data, the active verifier, family state,
remaining expiry and replacement uniqueness before advancing the family.

One concurrent refresh succeeds. Other uses reject without disclosing a reusable
replacement credential. Reuse during ReplayGrace (default 5s) rejects; reuse after
that grace revokes the family. A delayed legitimate retry can also revoke it. This
is a policy for reuse, not proof of theft and not a recovery mechanism.

FamilyTTL bounds the overall login family; each new refresh session is also
bounded by RefreshTTL and remaining family time. Used-token records expire with
the family. Their count grows with refresh traffic during that lifetime, so
rate-limit refresh endpoints. Access JWTs remain independent of family revocation.

Redis atomic execution cannot guarantee that a client receives the new secret.
If the connection fails after commit, the client may need to log in again. Signing,
serialization and collision failures do not consume the old credential. Lua does
not roll back arbitrary runtime/OOM failures: provision memory and correct ACL
permissions. Returned storage errors must not be treated as successful refreshes.

### Compatibility and errors

CookieSecure still defaults to false and empty AllowedOrigins retains legacy CSRF
behavior. Set CookieSecure=true and explicit origins for deployed cookie flows.
Audience is optional for compatibility; enabling it invalidates old audience-less
access tokens. Old access tokens without token_type remain accepted, while a
present incorrect type is rejected. HS256, expiry and issuer are always checked.

ErrInvalidAccessToken and ErrInvalidRefreshToken identify hybrid credential
failures. They are **not** transport.ErrInvalid. ErrMissingAccessToken remains
exported, but middleware responds directly rather than returning it. Refresh
maps any extraction failure to ErrMissingRefreshToken, including malformed,
conflicting or CSRF-rejected input. Storage/serialization failures are returned
separately. HTTP handlers should expose opaque messages/statuses.

Legacy standalone refresh records can migrate into the new format, bounded by
their remaining TTL, without prior replay history. The old key is deleted on
migration. Legacy Redis Cluster keys cannot all migrate in one slot; those users
must log in again. New tokens cannot be refreshed by older library versions.
Adding Config fields preserves keyed literals but can break unkeyed literals.

## Browser and mobile delivery

Use [transport configuration](configuration.md#auth-transport) for cookie names,
paths and exact trusted origins. Stateful Login handles its session cookie.
Single/stateless managers issue strings, so use transport.SetCookie explicitly
if you want browser delivery. Stateless refresh should use a separately named
refresh cookie/config, Extract it at the refresh endpoint, then call Refresh(raw).
The Transport field on stateless.Config configures access middleware only.

Mobile clients send the credential expected by that endpoint as a bearer token.
Do not combine an access bearer token with a different refresh cookie on a
refresh/logout endpoint: conflicting credentials are rejected. See the
[cookie/security rules](security.md#cookies-and-bearer-credentials) before
cross-origin browser integration.
