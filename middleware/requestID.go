package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const requestIdHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIdHeader)
		if id == "" {
			id = newRequestID()
		}
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
