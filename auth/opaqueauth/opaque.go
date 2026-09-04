package opaqueauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// --- CONTRACTS ---

// Session represents the internal session state saved by the Store.
type Session[T any] struct {
	UserID           string    `json:"user_id"`
	AccessTokenHash  string    `json:"access_token_hash"`
	RefreshTokenHash string    `json:"refresh_token_hash"`
	Payload          T         `json:"payload"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// TokenPair represents the unhashed plain-text tokens handed back to the caller.
type TokenPair[T any] struct {
	RawAccessToken  string    `json:"access_token"`
	RawRefreshToken string    `json:"refresh_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	Payload         T         `json:"payload"`
}

// Store defines the storage engine contract.
// Implementations (Redis, Postgres, Memory) only ever see SHA-256 hashed tokens.
type Store[T any] interface {
	Save(ctx context.Context, session Session[T]) error
	GetByAccessHash(ctx context.Context, hash string) (Session[T], error)
	GetByRefreshHash(ctx context.Context, hash string) (Session[T], error)
	RevokeByAccessHash(ctx context.Context, hash string) error
	RevokeByRefreshHash(ctx context.Context, hash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

// Manager defines the primary public authentication API.
type Manager[T any] interface {
	Issue(ctx context.Context, userID string, payload T) (TokenPair[T], error)
	Authenticate(ctx context.Context, rawAccessToken string) (T, error)
	Refresh(ctx context.Context, rawRefreshToken string) (TokenPair[T], error)
	Revoke(ctx context.Context, rawAccessToken string) error
	RevokeAll(ctx context.Context, userID string) error
}

// Config defines lifecycle parameters for opaque token management.
type Config struct {
	AccessTTL       time.Duration // Duration access token remains valid (e.g., 15m)
	RefreshTTL      time.Duration // Duration refresh token remains valid (e.g., 7d)
           // Byte length of raw entropy prior to hex encoding (Default: 32)
}



type manager[T any] struct {
	store Store[T]
	cfg   Config
}

// New constructs an encapsulated opaque session Manager.
func New[T any](store Store[T], cfg Config) Manager[T] {

	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}

	return &manager[T]{
		store: store,
		cfg:   cfg,
	}
}

func (m *manager[T]) Issue(ctx context.Context, userID string, payload T) (TokenPair[T], error) {
	rawAccess, err := generateCryptoToken(32)
	if err != nil {
		return TokenPair[T]{}, err
	}

	rawRefresh, err := generateCryptoToken(32)
	if err != nil {
		return TokenPair[T]{}, err
	}

	now := time.Now().UTC()
	accessExp := now.Add(m.cfg.AccessTTL)
	refreshExp := now.Add(m.cfg.RefreshTTL)

	session := Session[T]{
		UserID:           userID,
		AccessTokenHash:  hashToken(rawAccess),
		RefreshTokenHash: hashToken(rawRefresh),
		Payload:          payload,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
	}

	if err := m.store.Save(ctx, session); err != nil {
		return TokenPair[T]{}, err
	}

	return TokenPair[T]{
		RawAccessToken:  rawAccess,
		RawRefreshToken: rawRefresh,
		AccessExpiresAt: accessExp,
		RefreshExpiresAt: refreshExp,
		Payload:         payload,
	}, nil
}

func (m *manager[T]) Authenticate(ctx context.Context, rawAccessToken string) (T, error) {
	var zero T
	if rawAccessToken == "" {
		return zero, ErrInvalidToken
	}

	hash := hashToken(rawAccessToken)
	session, err := m.store.GetByAccessHash(ctx, hash)
	if err != nil {
		return zero, err
	}

	if time.Now().UTC().After(session.AccessExpiresAt) {
		_ = m.store.RevokeByAccessHash(ctx, hash)
		return zero, ErrExpiredToken
	}

	return session.Payload, nil
}

func (m *manager[T]) Refresh(ctx context.Context, rawRefreshToken string) (TokenPair[T], error) {
	if rawRefreshToken == "" {
		return TokenPair[T]{}, ErrInvalidToken
	}

	hash := hashToken(rawRefreshToken)
	session, err := m.store.GetByRefreshHash(ctx, hash)
	if err != nil {
		return TokenPair[T]{}, err
	}

	if time.Now().UTC().After(session.RefreshExpiresAt) {
		_ = m.store.RevokeByRefreshHash(ctx, hash)
		return TokenPair[T]{}, ErrExpiredToken
	}

	// Token Rotation: Revoke previous session state upon refresh
	_ = m.store.RevokeByRefreshHash(ctx, hash)

	// Re-issue a fresh token pair maintaining the existing payload
	return m.Issue(ctx, session.UserID, session.Payload)
}

func (m *manager[T]) Revoke(ctx context.Context, rawAccessToken string) error {
	if rawAccessToken == "" {
		return nil
	}
	return m.store.RevokeByAccessHash(ctx, hashToken(rawAccessToken))
}

func (m *manager[T]) RevokeAll(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	return m.store.RevokeAllForUser(ctx, userID)
}

//hash using sha256 
func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func generateCryptoToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}