package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response defines the standardized JSON envelope for all API endpoints.
type response[T any] struct {
	Success bool       `json:"success"`
	Data    T          `json:"data,omitempty"`
	Error   *errorItem `json:"error,omitempty"`
}

// ErrorItem provides a machine-readable error code alongside human-readable details.
type errorItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// JSON sends a success response wrapped in the standard envelope.
func json[T any](c *gin.Context, status int, data T) {
	c.JSON(status, response[T]{
		Success: true,
		Data:    data,
	})
}

// OK sends a 200 OK response wrapped in the standard envelope.
func OK[T any](c *gin.Context, data T) {
	json(c, http.StatusOK, data)
}

// Created sends a 201 Created response wrapped in the standard envelope.
func Created[T any](c *gin.Context, data T) {
	json(c, http.StatusCreated, data)
}

// NoContent sends a 204 No Content response without a body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}