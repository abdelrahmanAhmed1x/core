package opaqueauth

import (
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	// PayloadContextKey is the key used to store and retrieve the payload in gin.Context.
	PayloadContextKey = "opaque_session_payload"
)

// Middleware returns a Gin HandlerFunc that authenticates requests strictly via HTTP cookies.
func Middleware[T any](mgr Manager[T], cookieName string) gin.HandlerFunc {
	if cookieName == "" {
		cookieName = "access_token"
	}

	return func(c *gin.Context) {
		rawToken, err := c.Cookie(cookieName)
		if err != nil || rawToken == "" {
			httpx.AbortUnauthorized(c, "Missing or invalid session cookie")
			return
		}

		payload, err := mgr.Authenticate(c.Request.Context(), rawToken)
		if err != nil {
			httpx.AbortUnauthorized(c, "Invalid or expired session token")
			return
		}

		c.Set(PayloadContextKey, payload)
		c.Next()
	}
}

// GetPayload safely retrieves the strongly-typed session payload from gin.Context.
func GetPayload[T any](c *gin.Context) (T, bool) {
	val, exists := c.Get(PayloadContextKey)
	if !exists {
		var zero T
		return zero, false
	}
	payload, ok := val.(T)
	return payload, ok
}

// MustGetPayload retrieves T or panics if missing (use on routes protected by Middleware).
func MustGetPayload[T any](c *gin.Context) T {
	payload, ok := GetPayload[T](c)
	if !ok {
		panic("opaque: session payload not found in context - ensure Middleware is attached")
	}
	return payload
}
