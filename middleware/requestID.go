package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/abdelrahmanAhmed1x/core/logger"
	"github.com/gin-gonic/gin"
)

const requestIdHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIdHeader)
		if existing := c.Writer.Header().Get(requestIdHeader); existing != "" {
			id = existing
		}
		if len(id) > 128 {
			id = ""
		}
		if id == "" {
			id = newRequestID()
		}
		c.Request = c.Request.WithContext(logger.ContextWithRequestID(c.Request.Context(), id))
		c.Header(requestIdHeader, id)
		c.Set(requestIdHeader, id)
		c.Next()
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("middleware: failed to generate request id")
	}
	return hex.EncodeToString(b[:])
}
