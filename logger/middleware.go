package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const HeaderXRequestID = "X-Request-ID"

// Middleware provides request-tracing and access logging for Gin.
func Middleware(l Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 1. Get or generate Request ID
		reqID := c.GetHeader(HeaderXRequestID)
		if reqID == "" {
			reqID = generateTraceID()
		}

		// 2. Attach Request ID to response headers & request Context
		c.Header(HeaderXRequestID, reqID)
		ctx := c.Request.Context()
		ctx = ContextWithRequestID(ctx, reqID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// 3. Log HTTP request outcome
		latency := time.Since(start)
		status := c.Writer.Status()

		args := []any{
			"status", status,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"ip", c.ClientIP(),
			"latency_ms", latency.Milliseconds(),
		}

		if len(c.Errors) > 0 {
			args = append(args, "errors", c.Errors.String())
		}

		switch {
		case status >= http.StatusInternalServerError:
			l.Error(c.Request.Context(), "HTTP Request Failed", args...)
		case status >= http.StatusBadRequest:
			l.Warn(c.Request.Context(), "HTTP Request Client Error", args...)
		default:
			l.Info(c.Request.Context(), "HTTP Request Handled", args...)
		}
	}
}

func ContextWithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, reqID)
}

func generateTraceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
