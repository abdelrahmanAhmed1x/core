package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

func Timeout(timeout time.Duration) gin.HandlerFunc {
	if timeout <= 0 {
		panic("middleware: timeout must be greater than 0")
	}

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			timeout,
		)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
