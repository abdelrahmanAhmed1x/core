package stateful

import (
	"context"
	"time"
)

// CreateRequest describes a new session. TokenHash and UserHash are lowercase
// SHA-256 hex digests; the store never receives raw credentials or user IDs.
// Payload is JSON encoded by Manager and must be preserved losslessly, including
// numeric precision.
// AbsoluteTTL is positive; IdleTTL is nonnegative, with zero disabling idle expiry.
type CreateRequest struct {
	Namespace   string
	TokenHash   string
	UserHash    string
	Payload     []byte
	AbsoluteTTL time.Duration
	IdleTTL     time.Duration
}

// SessionStore is the authoritative storage boundary for Manager. Implementations
// must be safe for concurrent use, honor context cancellation, isolate namespaces,
// and return infrastructure errors separately from ErrInvalid and ErrCollision.
// They must not authenticate using stale cached state. Returned payloads must be
// safe for the caller to read after the method returns.
//
// Every operation must be atomic relative to other operations in its namespace.
// In particular, a Create ordered before RevokeAll is invalidated; a Create
// ordered after it remains valid. Renew must never resurrect revoked sessions.
// A verification ordered after revocation commits must reject the session.
//
// Implementations own expiry enforcement and use an authoritative clock. Expired
// sessions must be rejected even if physical cleanup has not run. Missing or
// corrupt revocation state must fail closed. Managers sharing a namespace must
// use the same AbsoluteTTL so retention of revocation state covers all sessions.
type SessionStore interface {
	// Create inserts without overwriting an existing credential. On collision it
	// returns ErrCollision without mutation. Absolute expiry is fixed at creation;
	// initial idle expiry is capped by absolute expiry. Copy Payload if retaining
	// it after the call. The manager retries collisions with fresh credentials.
	Create(ctx context.Context, req CreateRequest) error

	// Verify returns the original JSON payload of a valid session without moving
	// its expiry. Missing, expired, revoked, or corrupt sessions return ErrInvalid.
	Verify(ctx context.Context, namespace, tokenHash string) ([]byte, error)

	// Renew verifies and extends idle expiry atomically, capped by the original
	// absolute expiry. Zero idleTTL performs verification without renewal.
	// Invalid sessions return ErrInvalid, as with Verify.
	Renew(ctx context.Context, namespace, tokenHash string, idleTTL time.Duration) ([]byte, error)

	// Revoke invalidates one session. Missing sessions are a successful no-op.
	Revoke(ctx context.Context, namespace, tokenHash string) error

	// RevokeAll invalidates this user's earlier sessions in the namespace.
	// absoluteTTL bounds their remaining lifetime and can be used to retain
	// revocation state. Later creations must remain valid.
	RevokeAll(ctx context.Context, namespace, userHash string, absoluteTTL time.Duration) error
}
