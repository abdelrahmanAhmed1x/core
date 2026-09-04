package httpx

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Format validation errors cleanly if it's a validator.ValidationErrors error
func formatBindError(err error) any {
	if ve, ok := err.(validator.ValidationErrors); ok {
		out := make(map[string]string)
		for _, fe := range ve {
			out[fe.Field()] = fe.Tag() // e.g., "Email": "required"
		}
		return out
	}
	return err.Error()
}

// BindJSON attempts to bind request JSON to struct T.
// On error, it automatically calls AbortBadRequest and returns (zeroValue, false).
func BindJSON[T any](c *gin.Context) (T, bool) {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortBadRequest(c, "Invalid request payload", formatBindError(err))
		var zero T
		return zero, false
	}
	return req, true
}

// BindURI binds path parameters (e.g., /users/:id/posts/:post_id) into a struct T.
// Struct fields must be tagged with `uri:"..."`.
// Automatically calls AbortBadRequest if binding/validation fails.
func BindURI[T any](c *gin.Context) (T, bool) {
	var req T
	if err := c.ShouldBindUri(&req); err != nil {
		AbortBadRequest(c, "Invalid path parameters", formatBindError(err))
		var zero T
		return zero, false
	}
	return req, true
}

// BindQuery binds URL query parameters (e.g., ?page=1&limit=20) into a struct T.
// Struct fields must be tagged with `form:"..."`.
// Automatically calls AbortBadRequest if binding/validation fails.
func BindQuery[T any](c *gin.Context) (T, bool) {
	var req T
	if err := c.ShouldBindQuery(&req); err != nil {
		AbortBadRequest(c, "Invalid query parameters", formatBindError(err))
		var zero T
		return zero, false
	}
	return req, true
}
