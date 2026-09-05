package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AbortWithError halts the request chain with a standardized JSON error envelope.
func abortWithError(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, response[any]{
		Success: false,
		Error: &errorItem{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// AbortBadRequest halts with 400 Bad Request.
func AbortBadRequest(c *gin.Context, message string, details any) {
	abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", message, details)
}

// AbortUnauthorized halts with 401 Unauthorized.
func AbortUnauthorized(c *gin.Context, message string) {
	abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

// AbortForbidden halts with 403 Forbidden.
func AbortForbidden(c *gin.Context, message string) {
	abortWithError(c, http.StatusForbidden, "FORBIDDEN", message, nil)
}
// AbortMethodNotAllowed halts with 405 Method Not Allowed.
func AbortMethodNotAllowed(c *gin.Context, message string) {
	abortWithError(
		c,
		http.StatusMethodNotAllowed,
		"METHOD_NOT_ALLOWED",
		message,
		nil,
	)
}
// AbortConflict halts with 409 Conflict.
func AbortConflict(c *gin.Context, message string, details any) {
	abortWithError(
		c,
		http.StatusConflict,
		"CONFLICT",
		message,
		details,
	)
}
// AbortUnprocessableEntity halts with 422 Unprocessable Entity.
func AbortUnprocessableEntity(c *gin.Context, message string, details any) {
	abortWithError(
		c,
		http.StatusUnprocessableEntity,
		"UNPROCESSABLE_ENTITY",
		message,
		details,
	)
}

// AbortTooManyRequests halts with 429 Too Many Requests.
func AbortTooManyRequests(c *gin.Context) {
	abortWithError(
		c,
		http.StatusTooManyRequests,
		"TOO_MANY_REQUESTS",
		"Rate limit exceeded",
		nil,
	)
}

// AbortGatewayTimeout halts with 504 Gateway Timeout.
func AbortGatewayTimeout(c *gin.Context, message string) {
	abortWithError(
		c,
		http.StatusGatewayTimeout,
		"GATEWAY_TIMEOUT",
		message,
		nil,
	)
}

// AbortNotFound halts with 404 Not Found.
func AbortNotFound(c *gin.Context, message string) {
	abortWithError(c, http.StatusNotFound, "NOT_FOUND", message, nil)
}

// AbortInternal logs the internal error safely and halts with 500 without leaking details.
func AbortInternal(c *gin.Context, err error) {
	if err != nil {
		slog.Error("Internal server error", "error", err, "path", c.Request.URL.Path)
	}
	abortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", nil)
}