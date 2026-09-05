package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func MaxBodySize(bytes int64) gin.HandlerFunc {
	if bytes <= 0 {
		panic("middleware: max body size must be greater than 0")
	}

	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(
			c.Writer,
			c.Request.Body,
			bytes,
		)

		c.Next()
	}
}