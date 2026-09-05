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
	"net/http"
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
		tokenString, err := c.Cookie(accessCookieName)
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

		c.Next()
	}
}

func (m *manager[T]) OptionalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(accessCookieName)
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

		c.Next()
	}
}

func (m *manager[T]) parseAccessToken(tokenString string) (T, error) {
	var zero T

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
	)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrInvalidAccessToken, err)
	}

	if !token.Valid {
		return zero, ErrInvalidAccessToken
	}

	return tokenClaims.Payload, nil
}

func createAccessToken[T any](
	payload T,
	ttl time.Duration,
	issuer string,
	secret []byte,
) (string, error) {
	now := time.Now()

	tokenClaims := claims[T]{
		Payload: payload,
		RegisteredClaims: golangjwt.RegisteredClaims{
			Issuer:    issuer,
			IssuedAt:  golangjwt.NewNumericDate(now),
			ExpiresAt: golangjwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := golangjwt.NewWithClaims(
		golangjwt.SigningMethodHS256,
		tokenClaims,
	)

	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("jwt: sign access token: %w", err)
	}

	return signed, nil
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
func (m *manager[T]) storeRefresh(
	ctx context.Context,
	token string,
	payload T,
) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jwt: marshal refresh payload: %w", err)
	}

	if err := m.redis.Set(
		ctx,
		m.refreshKey(token),
		data,
		m.cfg.RefreshTTL,
	).Err(); err != nil {
		return fmt.Errorf("jwt: store refresh token: %w", err)
	}

	return nil
}

// consumeRefresh atomically retrieves and deletes a refresh session.
//
// GETDEL means two concurrent refresh requests using the same token cannot
// both succeed.
func (m *manager[T]) consumeRefresh(
	ctx context.Context,
	token string,
) (T, error) {
	var zero T

	data, err := m.redis.GetDel(
		ctx,
		m.refreshKey(token),
	).Bytes()

	if errors.Is(err, redis.Nil) {
		return zero, ErrInvalidRefreshToken
	}

	if err != nil {
		return zero, fmt.Errorf("jwt: consume refresh token: %w", err)
	}

	var payload T
	if err := json.Unmarshal(data, &payload); err != nil {
		return zero, fmt.Errorf("jwt: unmarshal refresh payload: %w", err)
	}

	return payload, nil
}

func (m *manager[T]) deleteRefresh(
	ctx context.Context,
	token string,
) error {
	if token == "" {
		return nil
	}

	if err := m.redis.Del(
		ctx,
		m.refreshKey(token),
	).Err(); err != nil {
		return fmt.Errorf("jwt: delete refresh token: %w", err)
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
	access, err := createAccessToken(
		payload,
		m.cfg.AccessTTL,
		m.cfg.Issuer,
		[]byte(m.cfg.SecretKey),
	)
	if err != nil {
		return err
	}

	refresh, err := generateRefreshToken()
	if err != nil {
		return err
	}

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
		m.cfg.RefreshTTL,
		m.cfg.RefreshTokenPath,
		m.cfg.CookieDomain,
		m.cfg.CookieSecure,
		m.cfg.CookieSameSite,
	)

	return nil
}

// Refresh consumes the current refresh token and rotates it.
//
// Once consumeRefresh succeeds, the old token is permanently invalid.
// If creating/storing the replacement token then fails, the user must log
// in again. This intentionally fails secure.
func (m *manager[T]) Refresh(c *gin.Context) error {
	refresh, err := c.Cookie(refreshCookieName)
	if err != nil {
		return ErrMissingRefreshToken
	}

	payload, err := m.consumeRefresh(
		c.Request.Context(),
		refresh,
	)
	if err != nil {
		return err
	}

	return m.Login(c, payload)
}

// Logout revokes the current refresh session and clears both cookies.
//
// Cookies are cleared even when Redis revocation fails so the local browser
// still loses its credentials. The Redis error is returned to the caller.
func (m *manager[T]) Logout(c *gin.Context) error {
	refresh, cookieErr := c.Cookie(refreshCookieName)

	var revokeErr error

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
