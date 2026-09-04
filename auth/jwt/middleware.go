package jwt

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	accessTokenCookie  = "access_token"
	refreshTokenCookie = "refresh_token"
	payloadContextKey  = "jwt_payload_context_key"
)

func (m *manager[T]) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(accessTokenCookie)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing access token"})
			return
		}

		claims, err := m.parseToken(cookie, "access")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(payloadContextKey, claims.Payload)
		c.Next()
	}
}

func (m *manager[T]) Login(c *gin.Context, payload T) error {
	accToken, err := m.createToken(payload, "access", m.cfg.AccessTTL)
	if err != nil {
		return err
	}

	refToken, err := m.createToken(payload, "refresh", m.cfg.RefreshTTL)
	if err != nil {
		return err
	}

	// Set Access Token Cookie (Path: /)
	setCookie(c, accessTokenCookie, accToken, int(m.cfg.AccessTTL.Seconds()), m.cfg.CookiePath, m.cfg.CookieDomain, m.cfg.CookieSecure, m.cfg.CookieSameSite)

	// Set Refresh Token Cookie (Path restricted to /refresh to reduce header bloat)
	setCookie(c, refreshTokenCookie, refToken, int(m.cfg.RefreshTTL.Seconds()), m.cfg.RefreshTokenPath, m.cfg.CookieDomain, m.cfg.CookieSecure, m.cfg.CookieSameSite)

	return nil
}

func (m *manager[T]) Refresh(c *gin.Context) error {
	cookie, err := c.Cookie(refreshTokenCookie)
	if err != nil || cookie == "" {
		return errors.New("missing refresh token cookie")
	}

	claims, err := m.parseToken(cookie, "refresh")
	if err != nil {
		return err
	}

	return m.Login(c, claims.Payload)
}

func (m *manager[T]) Logout(c *gin.Context) {
	// Clear Access Cookie
	setCookie(c, accessTokenCookie, "", -1, m.cfg.CookiePath, m.cfg.CookieDomain, m.cfg.CookieSecure, m.cfg.CookieSameSite)
	// Clear Refresh Cookie
	setCookie(c, refreshTokenCookie, "", -1, m.cfg.RefreshTokenPath, m.cfg.CookieDomain, m.cfg.CookieSecure, m.cfg.CookieSameSite)
}

// Low-level helper to support SameSite setting natively in Gin
func setCookie(c *gin.Context, name, value string, maxAge int, path, domain string, secure bool, sameSite http.SameSite) {
	c.SetSameSite(sameSite)
	c.SetCookie(name, value, maxAge, path, domain, secure, true)
}

// ExtractPayload retrieves the strongly-typed payload T from gin.Context.
func ExtractPayload[T any](c *gin.Context) (T, error) {
	val, exists := c.Get(payloadContextKey)
	if !exists {
		var zero T
		return zero, errors.New("payload not found in context")
	}

	payload, ok := val.(T)
	if !ok {
		var zero T
		return zero, errors.New("invalid payload type")
	}

	return payload, nil
}

// MustExtractPayload retrieves payload or panics if missing (useful for guaranteed protected routes).
func MustExtractPayload[T any](c *gin.Context) T {
	payload, err := ExtractPayload[T](c)
	if err != nil {
		panic("jwt middleware was not executed on this route: " + err.Error())
	}
	return payload
}