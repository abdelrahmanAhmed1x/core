package jwt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"net/http"
	"strings"
	"time"

	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
	golangjwt "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// Middleware validates the access JWT and stores its strongly typed payload
// in the Gin context.
func (m *manager[T]) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := m.extract(c, accessCookieName)
		if err != nil {
			httpx.AbortUnauthorized(c, "Authentication required")
			return
		}

		payload, err := m.parseAccessToken(tokenString)
		if err != nil {
			httpx.AbortUnauthorized(c, "Invalid or expired access token")
			return
		}

		c.Set(payloadContextKey, payload)
		transport.StorePayload(c, payload)

		c.Next()
	}
}

func (m *manager[T]) OptionalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := m.extract(c, accessCookieName)
		if errors.Is(err, transport.ErrConflict) || errors.Is(err, transport.ErrCSRF) {
			httpx.AbortUnauthorized(c, "Invalid credential transport")
			return
		}
		if err != nil {
			c.Next()
			return
		}

		payload, err := m.parseAccessToken(token)
		if err != nil {
			c.Next()
			return
		}

		c.Set(payloadContextKey, payload)
		transport.StorePayload(c, payload)

		c.Next()
	}
}

func (m *manager[T]) parseAccessToken(tokenString string) (T, error) {
	var zero T
	if len(tokenString) > 16384 {
		return zero, ErrInvalidAccessToken
	}

	tokenClaims := &claims[T]{}

	token, err := golangjwt.ParseWithClaims(
		tokenString,
		tokenClaims,
		func(token *golangjwt.Token) (any, error) {
			return []byte(m.cfg.SecretKey), nil
		},
		golangjwt.WithValidMethods([]string{
			golangjwt.SigningMethodHS256.Alg(),
		}),
		golangjwt.WithIssuer(m.cfg.Issuer),
		golangjwt.WithExpirationRequired(),
		golangjwt.WithIssuedAt(),
	)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrInvalidAccessToken, err)
	}

	if !token.Valid || (tokenClaims.Type != "" && tokenClaims.Type != "access") || (m.cfg.Audience != "" && !containsAudience(tokenClaims.Audience, m.cfg.Audience)) {
		return zero, ErrInvalidAccessToken
	}

	return tokenClaims.Payload, nil
}

// generateRefreshToken returns a 256-bit cryptographically secure opaque token.
func generateRefreshToken() (string, error) {
	var token [32]byte

	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("jwt: generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *manager[T]) refreshKey(token string) string {
	return "auth:" +
		m.cfg.Issuer +
		":refresh:" +
		hashRefreshToken(token)
}

// storeRefresh stores only the hash of the actual refresh token as part
// of the Redis key. Redis never receives the usable refresh credential.
func (m *manager[T]) storeRefresh(ctx context.Context, token string, payload T) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	keys := m.familyKeys(token)
	n, err := familyCreate.Run(ctx, m.redis, keys, data, m.familyTTL().Milliseconds(), hashRefreshToken(token), m.cfg.RefreshTTL.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("jwt: persist family: %w", err)
	}
	if n != 1 {
		return ErrInvalidRefreshToken
	}
	return nil
}

func (m *manager[T]) deleteRefresh(
	ctx context.Context,
	token string,
) error {
	if token == "" {
		return nil
	}

	key := m.familyKeys(token)[1]
	if !strings.Contains(token, ".") {
		key = m.refreshKey(token)
		return m.redis.Del(ctx, key).Err()
	}
	if err := m.redis.SetArgs(ctx, key, "REVOKED", redis.SetArgs{Mode: "XX", KeepTTL: true}).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	return nil
}

func setCookie(
	c *gin.Context,
	name string,
	value string,
	ttl time.Duration,
	path string,
	domain string,
	secure bool,
	sameSite http.SameSite,
) {
	c.SetSameSite(sameSite)

	c.SetCookie(
		name,
		value,
		int(ttl/time.Second),
		path,
		domain,
		secure,
		true, // HttpOnly
	)
}

func clearCookie(
	c *gin.Context,
	name string,
	path string,
	domain string,
	secure bool,
	sameSite http.SameSite,
) {
	c.SetSameSite(sameSite)

	c.SetCookie(
		name,
		"",
		-1,
		path,
		domain,
		secure,
		true,
	)
}

// Login creates both credentials.
//
// The refresh session is persisted before either cookie is sent. This avoids
// handing the client a refresh token that the server failed to store.
func (m *manager[T]) Login(
	c *gin.Context,
	payload T,
) error {
	access, err := m.signAccess(payload)
	if err != nil {
		return err
	}

	refresh, err := generateRefreshToken()
	if err != nil {
		return err
	}

	family, err := generateRefreshToken()
	if err != nil {
		return err
	}
	refresh = family + "." + refresh

	if err := m.storeRefresh(
		c.Request.Context(),
		refresh,
		payload,
	); err != nil {
		return err
	}

	setCookie(
		c,
		accessCookieName,
		access,
		m.cfg.AccessTTL,
		m.cfg.CookiePath,
		m.cfg.CookieDomain,
		m.cfg.CookieSecure,
		m.cfg.CookieSameSite,
	)

	setCookie(
		c,
		refreshCookieName,
		refresh,
		m.familyTTL(),
		m.cfg.RefreshTokenPath,
		m.cfg.CookieDomain,
		m.cfg.CookieSecure,
		m.cfg.CookieSameSite,
	)

	return nil
}

// Refresh consumes the current refresh token and rotates it.
//
// Signing and serialization finish before the atomic Redis transition.
// Delivery can still be lost if the network fails after Redis commits.
func (m *manager[T]) Refresh(c *gin.Context) error {
	old, err := m.extract(c, refreshCookieName)
	if err != nil {
		return ErrMissingRefreshToken
	}
	parts := strings.Split(old, ".")
	if len(parts) > 2 {
		return ErrInvalidRefreshToken
	}
	for _, p := range parts {
		if !token.ValidOpaque(p) {
			return ErrInvalidRefreshToken
		}
	}
	ctx := c.Request.Context()
	keys := m.familyKeys(old)
	data, err := m.redis.Get(ctx, keys[0]).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrInvalidRefreshToken
	}
	if err != nil {
		return err
	}
	if strings.HasPrefix(string(data), "USED:") {
		_, err = familyReplay.Run(ctx, m.redis, keys, m.replayGrace().Milliseconds()).Result()
		if err != nil {
			return err
		}
		return ErrInvalidRefreshToken
	}
	var payload T
	if err = json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("jwt: corrupt refresh state: %w", err)
	}
	access, err := m.signAccess(payload)
	if err != nil {
		return err
	}
	secret, err := generateRefreshToken()
	if err != nil {
		return err
	}
	replacement := secret
	if parts := strings.Split(old, "."); len(parts) == 2 {
		replacement = parts[0] + "." + secret
	} else {
		family, e := generateRefreshToken()
		if e != nil {
			return e
		}
		replacement = family + "." + secret
	}
	next := m.familyKeys(replacement)
	n, err := familyRotate.Run(ctx, m.redis, []string{keys[0], next[0], keys[1], next[1]}, data, hashRefreshToken(old), hashRefreshToken(replacement), m.cfg.RefreshTTL.Milliseconds(), strings.Contains(old, ".")).Int()
	if err != nil {
		return fmt.Errorf("jwt: atomic rotation: %w", err)
	}
	if n != 1 {
		return ErrInvalidRefreshToken
	}
	m.deliver(c, access, replacement)
	return nil
}

// Logout revokes the current refresh session and clears both cookies.
//
// Cookies are cleared even when Redis revocation fails so the local browser
// still loses its credentials. The Redis error is returned to the caller.
func (m *manager[T]) Logout(c *gin.Context) error {
	refresh, cookieErr := m.extract(c, refreshCookieName)

	var revokeErr error
	if cookieErr != nil && !errors.Is(cookieErr, transport.ErrMissing) {
		revokeErr = cookieErr
	}

	if cookieErr == nil {
		revokeErr = m.deleteRefresh(
			c.Request.Context(),
			refresh,
		)
	}

	clearCookie(
		c,
		accessCookieName,
		m.cfg.CookiePath,
		m.cfg.CookieDomain,
		m.cfg.CookieSecure,
		m.cfg.CookieSameSite,
	)

	clearCookie(
		c,
		refreshCookieName,
		m.cfg.RefreshTokenPath,
		m.cfg.CookieDomain,
		m.cfg.CookieSecure,
		m.cfg.CookieSameSite,
	)

	return revokeErr
}

// ExtractPayload retrieves the authenticated payload placed into the
// Gin context by Middleware.
func (m *manager[T]) Payload(c *gin.Context) (T, bool) {
	var zero T

	value, exists := c.Get(payloadContextKey)
	if !exists {
		return zero, false
	}

	payload, ok := value.(T)
	if !ok {
		return zero, false
	}

	return payload, true
}

// MustExtractPayload retrieves the authenticated payload and panics if the
// authentication middleware has not populated it or T is incorrect.
//
// Use only on routes protected by Authenticator.Middleware().
func (m *manager[T]) MustPayload(c *gin.Context) T {
	payload, ok := m.Payload(c)
	if !ok {
		panic("jwt: authenticated payload missing from context")
	}

	return payload
}
