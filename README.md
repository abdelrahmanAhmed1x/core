# Core

`core` is a lightweight, production-grade Go toolkit designed for building robust, scalable web applications and microservices with [Gin](https://github.com/gin-gonic/gin). It provides generic HTTP response envelopes, automatic request binding and validation, context-aware `slog` logging with sensitive data masking, Redis-backed JWT cookie authentication with atomic refresh rotation, distributed rate limiting, HTTP security middlewares, and generic pagination utilities.

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Package Overview](#package-overview)
- [Packages & Usage](#packages--usage)
  - [1. `httpx` — Standardized HTTP Responses, Request Binders & Errors](#1-httpx--standardized-http-responses-request-binders--errors)
  - [2. `auth/jwt` — Redis-Backed Cookie JWT Authenticator](#2-authjwt--redis-backed-cookie-jwt-authenticator)
  - [3. `logger` — Context-Aware Structured Logging & Tracing](#3-logger--context-aware-structured-logging--tracing)
  - [4. `middleware` — Production HTTP Security & Control Middlewares](#4-middleware--production-http-security--control-middlewares)
  - [5. `pagination` — Generic Request & Response Pagination](#5-pagination--generic-request--response-pagination)
  - [6. `ratelimiter` — Redis-Backed Distributed Rate Limiting](#6-ratelimiter--redis-backed-distributed-rate-limiting)
- [Complete Full-Server Example](#complete-full-server-example)
- [License](#license)

---

## Features

- **Generic HTTP Envelopes (`httpx`)**: Consistent JSON structure for success and error responses across all APIs.
- **Auto Binders & Validation (`httpx`)**: Generic helpers for JSON, URI, and Query string binding with automatic validation error formatting.
- **Secure JWT Auth (`auth/jwt`)**: HttpOnly cookie-based JWT access tokens coupled with 256-bit opaque refresh tokens stored in Redis with atomic `GETDEL` single-use rotation.
- **Context-Aware Logging (`logger`)**: `slog`-backed structured logger with request ID tracing, user context tracking, automatic sensitive key redacting (`password`, `token`, `secret`, etc.), and Gin access logging middleware.
- **Essential Security Middlewares (`middleware`)**: Configurable CORS, body size limits (`MaxBodySize`), security headers, request ID generation, and context timeout enforcement.
- **Type-Safe Pagination (`pagination`)**: Automatic binding for `page` & `limit`, database offset calculation (`Offset()`), and generic response result wrapping with total page math.
- **Redis Rate Limiting (`ratelimiter`)**: Sliding window rate limiting powered by `go-redis/redis_rate` with custom key extractors (e.g., IP, User ID) and configurable fail-open / fail-closed modes.

---

## Installation

```bash
go get github.com/abdelrahmanAhmed1x/core
```

**Requirements:** Go 1.26 or higher (uses Go generics and standard library `log/slog`).

---

## Package Overview

| Package | Import Path | Description |
| :--- | :--- | :--- |
| **`httpx`** | `github.com/abdelrahmanAhmed1x/core/httpx` | Standardized JSON response envelopes, generic request binders (JSON, URI, Query), and HTTP error abort helpers. |
| **`auth/jwt`** | `github.com/abdelrahmanAhmed1x/core/auth/jwt` | Cookie-based JWT authenticator with Redis-backed opaque refresh token rotation and generic payload support. |
| **`logger`** | `github.com/abdelrahmanAhmed1x/core/logger` | `slog`-backed structured logger with request tracing, automatic sensitive field redaction, and Gin middleware. |
| **`middleware`** | `github.com/abdelrahmanAhmed1x/core/middleware` | Essential HTTP middlewares: CORS, body size limit, security headers, request ID, and request timeout. |
| **`pagination`** | `github.com/abdelrahmanAhmed1x/core/pagination` | Generic pagination query binding (`page`, `limit`), database offset calculation, and paginated response builders. |
| **`ratelimiter`** | `github.com/abdelrahmanAhmed1x/core/ratelimiter` | Redis-backed sliding window rate limiter middleware supporting custom keys (e.g. IP, User ID) and fail-open/closed modes. |

---

## Packages & Usage

### 1. `httpx` — Standardized HTTP Responses, Request Binders & Errors

The `httpx` package ensures a consistent API JSON response structure across your application, eliminating boilerplate code for request parsing, validation, and error handling.

#### Standardized JSON Envelopes

**Success Response Envelope (`200 OK`, `201 Created`):**
```json
{
  "success": true,
  "data": {
    "id": "usr_123",
    "name": "John Doe"
  }
}
```

**Error Response Envelope (`400`, `401`, `404`, `500`, etc.):**
```json
{
  "success": false,
  "error": {
    "code": "BAD_REQUEST",
    "message": "Invalid request payload",
    "details": {
      "Email": "required",
      "Age": "min"
    }
  }
}
```

#### Code Example

```go
package main

import (
	"errors"

	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
)

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type UserURIParam struct {
	ID string `uri:"id" binding:"required"`
}

type UserSearchQuery struct {
	Query string `form:"q" binding:"required"`
}

func createUserHandler(c *gin.Context) {
	// 1. Bind and validate JSON body (automatically calls AbortBadRequest on validation error)
	req, ok := httpx.BindJSON[CreateUserRequest](c)
	if !ok {
		return // Request was automatically aborted with 400 Bad Request
	}

	// 2. Return 201 Created wrapped in standardized JSON envelope
	httpx.Created(c, gin.H{
		"id":    "usr_100",
		"name":  req.Name,
		"email": req.Email,
	})
}

func getUserHandler(c *gin.Context) {
	// Bind path parameters (e.g. GET /users/:id)
	params, ok := httpx.BindURI[UserURIParam](c)
	if !ok {
		return
	}

	if params.ID == "unknown" {
		httpx.AbortNotFound(c, "User not found")
		return
	}

	httpx.OK(c, gin.H{"id": params.ID, "name": "Alice"})
}

func searchUsersHandler(c *gin.Context) {
	// Bind URL query parameters (e.g. GET /users/search?q=alice)
	query, ok := httpx.BindQuery[UserSearchQuery](c)
	if !ok {
		return
	}

	httpx.OK(c, gin.H{"query": query.Query, "results": []string{"Alice"}})
}

func deleteUserHandler(c *gin.Context) {
	// Return 204 No Content (no body)
	httpx.NoContent(c)
}

func triggerErrorHandlers(c *gin.Context) {
	// Examples of other error abort helpers:
	_ = func() {
		httpx.AbortUnauthorized(c, "Authentication required")
		httpx.AbortForbidden(c, "Permission denied")
		httpx.AbortMethodNotAllowed(c, "Method not allowed on this endpoint")
		httpx.AbortConflict(c, "Email already in use", gin.H{"field": "email"})
		httpx.AbortUnprocessableEntity(c, "Cannot process payload", nil)
		httpx.AbortTooManyRequests(c)
		httpx.AbortGatewayTimeout(c, "Upstream service timeout")
		httpx.AbortInternal(c, errors.New("database connection failed")) // logs error safely, hides details from client
	}
}

func main() {
	r := gin.Default()
	r.POST("/users", createUserHandler)
	r.GET("/users/:id", getUserHandler)
	r.GET("/users/search", searchUsersHandler)
	r.DELETE("/users/:id", deleteUserHandler)
	r.Run(":8080")
}
```

#### API Reference

- **Success Responses**:
  - `httpx.OK[T](c *gin.Context, data T)` — Sends HTTP `200 OK` wrapped in `{"success": true, "data": T}`.
  - `httpx.Created[T](c *gin.Context, data T)` — Sends HTTP `201 Created` wrapped in `{"success": true, "data": T}`.
  - `httpx.NoContent(c *gin.Context)` — Sends HTTP `204 No Content` without a body.
- **Request Binders**:
  - `httpx.BindJSON[T](c *gin.Context) (T, bool)` — Binds JSON request body to `T`. Automatically aborts with HTTP `400` on failure.
  - `httpx.BindURI[T](c *gin.Context) (T, bool)` — Binds route URI parameters (tagged `uri:"..."`) to `T`. Automatically aborts with HTTP `400` on failure.
  - `httpx.BindQuery[T](c *gin.Context) (T, bool)` — Binds URL query string parameters (tagged `form:"..."`) to `T`. Automatically aborts with HTTP `400` on failure.
- **Error Abort Helpers**:
  - `httpx.AbortBadRequest(c, message, details)` — HTTP `400 Bad Request`
  - `httpx.AbortUnauthorized(c, message)` — HTTP `401 Unauthorized`
  - `httpx.AbortForbidden(c, message)` — HTTP `403 Forbidden`
  - `httpx.AbortNotFound(c, message)` — HTTP `404 Not Found`
  - `httpx.AbortMethodNotAllowed(c, message)` — HTTP `405 Method Not Allowed`
  - `httpx.AbortConflict(c, message, details)` — HTTP `409 Conflict`
  - `httpx.AbortUnprocessableEntity(c, message, details)` — HTTP `422 Unprocessable Entity`
  - `httpx.AbortTooManyRequests(c)` — HTTP `429 Too Many Requests`
  - `httpx.AbortGatewayTimeout(c, message)` — HTTP `504 Gateway Timeout`
  - `httpx.AbortInternal(c, err)` — HTTP `500 Internal Server Error` (logs the internal error via `slog.Error` and sends a generic client response).

---

### 2. `auth/jwt` — Redis-Backed Cookie JWT Authenticator

The `auth/jwt` package provides a secure, cookie-based JWT authentication manager backed by Redis. It issues short-lived JWT access tokens in HttpOnly cookies and stores SHA-256 hashed 256-bit opaque refresh tokens in Redis for atomic single-use rotation via `GETDEL`. It supports custom strongly-typed generic payload structs `T`.

#### Architecture & Security Features

- **Access Tokens**: Encoded as HS256 JWTs stored in `access_token` HttpOnly cookies.
- **Refresh Tokens**: High-entropy 256-bit random tokens stored in `refresh_token` HttpOnly cookies.
- **Redis Security**: Redis stores only the SHA-256 hash of refresh tokens (`auth:<issuer>:refresh:<hash>`).
- **Atomic Rotation**: Refresh operations call Redis `GETDEL` atomically. Once refreshed, the old refresh token is deleted immediately.

#### Code Example

```go
package main

import (
	"net/http"
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// UserSession defines the custom payload embedded inside JWT access tokens and Redis refresh sessions.
type UserSession struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func main() {
	// 1. Initialize Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// 2. Initialize JWT Authenticator
	authManager := jwt.New[UserSession](rdb, jwt.Config{
		SecretKey:        "super-secret-key-at-least-32-bytes-long!", // Min 32 bytes for HS256
		Issuer:           "my-app",
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       7 * 24 * time.Hour,
		CookieDomain:     "localhost",
		CookiePath:       "/",
		CookieSecure:     false, // Set true in production (HTTPS)
		CookieSameSite:   http.SameSiteLaxMode,
		RefreshTokenPath: "/auth", // Scope refresh cookie to /auth routes
	})

	r := gin.Default()

	// Auth group
	authGroup := r.Group("/auth")
	{
		// Login endpoint
		authGroup.POST("/login", func(c *gin.Context) {
			session := UserSession{
				UserID: "usr_42",
				Email:  "user@example.com",
				Role:   "admin",
			}

			// Login issues JWT access token and opaque refresh token in HttpOnly cookies
			if err := authManager.Login(c, session); err != nil {
				httpx.AbortInternal(c, err)
				return
			}

			httpx.OK(c, gin.H{"message": "Login successful"})
		})

		// Refresh token endpoint (rotates access and refresh tokens)
		authGroup.POST("/refresh", func(c *gin.Context) {
			if err := authManager.Refresh(c); err != nil {
				httpx.AbortUnauthorized(c, "Invalid or expired refresh token")
				return
			}

			httpx.OK(c, gin.H{"message": "Token refreshed successfully"})
		})

		// Logout endpoint
		authGroup.POST("/logout", func(c *gin.Context) {
			if err := authManager.Logout(c); err != nil {
				// Redis deletion error if any; cookies are still cleared safely
			}

			httpx.OK(c, gin.H{"message": "Logged out successfully"})
		})
	}

	// Strictly protected routes (requires valid access_token cookie)
	protected := r.Group("/api", authManager.Middleware())
	{
		protected.GET("/profile", func(c *gin.Context) {
			// Extract typed payload from context (panics if not authenticated)
			payload := authManager.MustPayload(c)

			httpx.OK(c, gin.H{
				"user_id": payload.UserID,
				"email":   payload.Email,
				"role":    payload.Role,
			})
		})
	}

	// Optional authentication routes (allows guests and authenticated users)
	optional := r.Group("/public")
	optional.Use(authManager.OptionalMiddleware())
	{
		optional.GET("/feed", func(c *gin.Context) {
			payload, ok := authManager.Payload(c)
			if !ok {
				httpx.OK(c, gin.H{"mode": "guest", "content": "Public Feed"})
				return
			}

			httpx.OK(c, gin.H{"mode": "authenticated", "user_id": payload.UserID})
		})
	}

	r.Run(":8080")
}
```

#### API Reference

- **Constructor**:
  - `jwt.New[T](client redis.UniversalClient, cfg jwt.Config) jwt.Authenticator[T]` — Instantiates a JWT authenticator. Panics if `client` is nil or `cfg` validation fails.
- **`Authenticator[T]` Interface**:
  - `Middleware() gin.HandlerFunc` — Gin middleware requiring valid `access_token` cookie. Aborts with `401 Unauthorized` if missing/invalid.
  - `OptionalMiddleware() gin.HandlerFunc` — Gin middleware attempting to parse `access_token`. Proceeds as guest if missing or invalid.
  - `Login(c *gin.Context, payload T) error` — Issues access and refresh tokens, saves refresh session hash in Redis, sets HttpOnly cookies.
  - `Refresh(c *gin.Context) error` — Atomically consumes existing refresh token from Redis and rotates token pair.
  - `Logout(c *gin.Context) error` — Deletes refresh token from Redis and clears both cookies.
  - `Payload(c *gin.Context) (T, bool)` — Retrieves typed payload from `gin.Context`.
  - `MustPayload(c *gin.Context) T` — Retrieves typed payload or panics if unpopulated.
- **`Config` Options**:
  - `SecretKey` *(string, required)* — Secret key for signing JWTs (must be at least 32 bytes).
  - `Issuer` *(string, required)* — JWT issuer claim string.
  - `AccessTTL` *(time.Duration)* — Access token lifespan (default: `15m`).
  - `RefreshTTL` *(time.Duration)* — Refresh token lifespan (default: `7d`).
  - `CookieDomain` *(string)* — Cookie domain.
  - `CookiePath` *(string)* — Cookie path (default: `"/"`).
  - `CookieSecure` *(bool)* — Requires HTTPS for cookies.
  - `CookieSameSite` *(http.SameSite)* — Cookie SameSite policy (default: `http.SameSiteLaxMode`).
  - `RefreshTokenPath` *(string)* — Path restriction for `refresh_token` cookie (default: `"/"`).
- **Exported Errors**:
  - `jwt.ErrMissingAccessToken`
  - `jwt.ErrInvalidAccessToken`
  - `jwt.ErrMissingRefreshToken`
  - `jwt.ErrInvalidRefreshToken`

---

### 3. `logger` — Context-Aware Structured Logging & Tracing

The `logger` package wraps standard library `log/slog` to provide structured logging with request ID tracing, user ID tracking, automatic sensitive data redaction (`password`, `token`, `secret`, etc.), and a Gin access-logging middleware.

#### Features

- **Context Metadata Extraction**: Log functions automatically pull `request_id` and `user_id` from `context.Context`.
- **Sensitive Key Redaction**: `maskingHandler` masks sensitive field keys (`password`, `token`, `secret`, `authorization`, `cookie`, `access_token`, `refresh_token`) into `"[REDACTED]"`.
- **Gin Access Middleware**: Automatically logs HTTP request method, path, IP, status code, latency (ms), and errors.

#### Code Example

```go
package main

import (
	"context"
	"os"

	"github.com/abdelrahmanAhmed1x/core/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize Logger
	log := logger.New(logger.Config{
		Level:          "debug",   // "debug", "info", "warn", "error" (default: "info")
		Format:         "json",    // "json" or "text" (default: "text")
		Output:         os.Stdout, // Default: os.Stdout
		MaskSensitives: true,      // Automatically redacts sensitive fields
	})

	r := gin.New()

	// 2. Attach Gin access logging & tracing middleware
	r.Use(logger.Middleware(log))

	r.POST("/login", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Context log output will automatically contain request_id from logger.Middleware
		log.Info(ctx, "User attempt login", "username", "john_doe")

		// Sensitive attributes are automatically masked in output as "[REDACTED]"
		log.Debug(ctx, "Authenticating user credentials",
			"username", "john_doe",
			"password", "my_super_secret_password", // -> "[REDACTED]"
			"token", "bearer-token-12345",          // -> "[REDACTED]"
		)

		// Create child logger with contextual attributes
		userLogger := log.With("module", "user_service")
		userLogger.Info(ctx, "User profile updated", "user_id", "usr_100")

		c.JSON(200, gin.H{"status": "ok"})
	})

	// Manual context creation helper example:
	ctx := context.Background()
	ctx = logger.ContextWithRequestID(ctx, "req-99999")
	log.Info(ctx, "Background task started") // Includes request_id="req-99999"

	r.Run(":8080")
}
```

#### API Reference

- **Constructor**:
  - `logger.New(cfg logger.Config) logger.Logger` — Constructs a `Logger` instance.
- **`Logger` Interface**:
  - `Debug(ctx context.Context, msg string, args ...any)`
  - `Info(ctx context.Context, msg string, args ...any)`
  - `Warn(ctx context.Context, msg string, args ...any)`
  - `Error(ctx context.Context, msg string, args ...any)`
  - `With(args ...any) Logger` — Returns child logger with pre-attached key-value attributes.
  - `Slog() *slog.Logger` — Exposes underlying standard `*slog.Logger`.
- **Gin Middleware & Helpers**:
  - `logger.Middleware(l logger.Logger) gin.HandlerFunc` — Gin middleware generating/propagating `X-Request-ID` and logging request metrics.
  - `logger.ContextWithRequestID(ctx context.Context, reqID string) context.Context` — Injects request ID into context.
- **Context Keys & Constants**:
  - `logger.RequestIDKey` (`"request_id"`)
  - `logger.UserIDKey` (`"user_id"`)
  - `logger.HeaderXRequestID` (`"X-Request-ID"`)

---

### 4. `middleware` — Production HTTP Security & Control Middlewares

The `middleware` package provides essential production middlewares for CORS configuration, body size limits, HTTP security headers, request ID injection, and context timeouts.

#### Code Example

```go
package main

import (
	"time"

	"github.com/abdelrahmanAhmed1x/core/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	r.Use(gin.Recovery())

	// 1. Request ID Middleware (Injects/propagates X-Request-ID header & context key)
	r.Use(middleware.RequestID())

	// 2. Security Headers Middleware (Injected headers: X-Content-Type-Options, X-Frame-Options, Referrer-Policy, X-Permitted-Cross-Domain-Policies)
	r.Use(middleware.SecurityHeaders())

	// 3. Max Body Size Middleware (Restricts request body to 10 MB to prevent memory exhaustion DoS)
	r.Use(middleware.MaxBodySize(10 * 1024 * 1024))

	// 4. Request Timeout Middleware (Applies 30-second deadline to c.Request.Context())
	r.Use(middleware.Timeout(30 * time.Second))

	// 5. Configurable CORS Middleware
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins:   []string{"https://example.com", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours preflight cache
	}))

	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.Run(":8080")
}
```

#### API Reference

- `middleware.CORS(cfg CORSConfig) gin.HandlerFunc` — Sets CORS response headers and handles HTTP `OPTIONS` preflight requests. Panics if `AllowedOrigins` is empty or wildcard `*` is used with `AllowCredentials: true`.
- `middleware.MaxBodySize(bytes int64) gin.HandlerFunc` — Wraps `c.Request.Body` with `http.MaxBytesReader`. Panics if `bytes <= 0`.
- `middleware.RequestID() gin.HandlerFunc` — Reads `X-Request-ID` header or generates a random 16-byte hex ID, setting it on response headers and context.
- `middleware.SecurityHeaders() gin.HandlerFunc` — Sets OWASP recommended defense-in-depth HTTP response headers.
- `middleware.Timeout(timeout time.Duration) gin.HandlerFunc` — Attaches a deadline context to `c.Request.Context()`. Panics if `timeout <= 0`.

---

### 5. `pagination` — Generic Request & Response Pagination

The `pagination` package handles binding pagination query parameters (`page`, `limit`), computing SQL offsets (`Offset()`), and returning structured JSON pagination metadata.

#### Code Example

```go
package main

import (
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/abdelrahmanAhmed1x/core/pagination"
	"github.com/gin-gonic/gin"
)

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func listProductsHandler(c *gin.Context) {
	// 1. Bind pagination query (?page=2&limit=10). Defaults to page=1, limit=10 if omitted.
	q, ok := httpx.BindQuery[pagination.Query](c)
	if !ok {
		return
	}

	// Calculate database query offset: (Page - 1) * Limit
	dbOffset := q.Offset()
	_ = dbOffset // Use in SQL: SELECT * FROM products LIMIT q.Limit OFFSET dbOffset

	// Mock DB result
	products := []Product{
		{ID: 11, Name: "Mechanical Keyboard", Price: 99.99},
		{ID: 12, Name: "Wireless Mouse", Price: 49.99},
	}
	totalProductsCount := 45

	// 2. Build generic pagination result
	result := pagination.NewResult(products, totalProductsCount, q)

	// 3. Send standardized OK response envelope
	httpx.OK(c, result)
}

func main() {
	r := gin.Default()
	r.GET("/products", listProductsHandler)
	r.Run(":8080")
}
```

#### JSON Output Example (`GET /products?page=2&limit=10`)

```json
{
  "success": true,
  "data": {
    "items": [
      { "id": 11, "name": "Mechanical Keyboard", "price": 99.99 },
      { "id": 12, "name": "Wireless Mouse", "price": 49.99 }
    ],
    "meta": {
      "page": 2,
      "limit": 10,
      "total_items": 45,
      "total_pages": 5
    }
  }
}
```

#### API Reference

- **`pagination.Query` Struct**:
  - `Page int` — `form:"page,default=1" binding:"omitempty,min=1"`
  - `Limit int` — `form:"limit,default=10" binding:"omitempty,min=1,max=100"`
  - `EnsureDefaults()` — Sets `Page = 1` if `<= 0` and `Limit = 10` if `<= 0`.
  - `Offset() int` — Returns calculated DB offset `(Page - 1) * Limit`.
- **`pagination.Meta` Struct**:
  - `Page int`, `Limit int`, `TotalItems int`, `TotalPages int`
- **`pagination.Result[T]` Struct**:
  - `Items []T`, `Meta pagination.Meta`
- **Constructor**:
  - `pagination.NewResult[T](items []T, totalItems int, q Query) Result[T]` — Calculates total pages (`ceil(totalItems / limit)`) and returns `Result[T]`. Ensures empty slice `[]T` instead of `null` in JSON when items is nil.

---

### 6. `ratelimiter` — Redis-Backed Distributed Rate Limiting

The `ratelimiter` package implements sliding-window rate limiting using Redis (`github.com/go-redis/redis_rate/v10`). It allows custom key generation (IP address, user ID, API key) and supports configurable failure modes (`FailOpen` or `FailClosed`).

#### Failure Modes

- **`ratelimiter.FailOpen`**: If Redis is unreachable, the rate limiter allows the request to pass through (prioritizes availability).
- **`ratelimiter.FailClosed`**: If Redis is unreachable, the rate limiter aborts with `503 Service Unavailable` (prioritizes protection).

#### Code Example

```go
package main

import (
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/ratelimiter"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type SessionPayload struct {
	UserID string `json:"user_id"`
}

func main() {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	limiter := ratelimiter.New(rdb)

	r := gin.Default()

	// 1. IP-based rate limiting (100 requests per minute per IP, FailOpen mode)
	ipLimitMiddleware := limiter.Middleware(
		ratelimiter.Limit{
			Rate:        100,
			Burst:       100,
			Period:      time.Minute,
			FailureMode: ratelimiter.FailOpen,
		},
		ratelimiter.ByIP, // Builtin KeyFunc returning c.ClientIP()
	)

	// 2. Custom User-ID or Header rate limiting
	userLimitMiddleware := limiter.Middleware(
		ratelimiter.Limit{
			Rate:        10,
			Burst:       20,
			Period:      time.Minute,
			FailureMode: ratelimiter.FailClosed,
		},
		func(c *gin.Context) string {
			// Extract custom key, e.g. API key or authenticated User ID
			if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
				return "rate:api_key:" + apiKey
			}
			return "rate:ip:" + c.ClientIP()
		},
	)

	r.GET("/public-api", ipLimitMiddleware, func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.POST("/sensitive-op", userLimitMiddleware, func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "processed"})
	})

	r.Run(":8080")
}
```

#### API Reference

- **Constructor**:
  - `ratelimiter.New(client redis.UniversalClient) *limiter` — Instantiates rate limiter.
- **Methods**:
  - `Middleware(limit Limit, keyfn KeyFunc) gin.HandlerFunc` — Returns Gin middleware enforcing rate limits. Automatically calls `httpx.AbortTooManyRequests(c)` when limit is exceeded.
- **`Limit` Struct**:
  - `Rate int` — Number of allowed events per period.
  - `Burst int` — Maximum burst size.
  - `Period time.Duration` — Time window duration (e.g. `time.Minute`).
  - `FailureMode FailureMode` — `ratelimiter.FailOpen` or `ratelimiter.FailClosed`.
- **Key Extractors & Enums**:
  - `ratelimiter.ByIP` — Built-in `KeyFunc` returning `c.ClientIP()`.
  - `ratelimiter.FailOpen` (`0`) — Allows requests on Redis failure.
  - `ratelimiter.FailClosed` (`1`) — Aborts with HTTP 503 on Redis failure.

---

## Complete Full-Server Example

Below is a complete, production-ready server demonstrating how all packages in `core` integrate together:

```go
package main

import (
	"net/http"
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/abdelrahmanAhmed1x/core/logger"
	"github.com/abdelrahmanAhmed1x/core/middleware"
	"github.com/abdelrahmanAhmed1x/core/pagination"
	"github.com/abdelrahmanAhmed1x/core/ratelimiter"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Data models
type UserAccount struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type CreateArticleRequest struct {
	Title   string `json:"title" binding:"required,min=3"`
	Content string `json:"content" binding:"required"`
}

type Article struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// 1. Infrastructure Setup: Redis & Logger
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	log := logger.New(logger.Config{
		Level:          "info",
		Format:         "json",
		MaskSensitives: true,
	})

	// 2. Authentication Setup
	auth := jwt.New[UserAccount](rdb, jwt.Config{
		SecretKey:        "production-super-secret-key-32bytes!",
		Issuer:           "core-demo-api",
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       7 * 24 * time.Hour,
		CookieSecure:     false,
		CookieSameSite:   http.SameSiteLaxMode,
		RefreshTokenPath: "/auth",
	})

	// 3. Rate Limiter Setup
	limiter := ratelimiter.New(rdb)

	// 4. Gin Engine & Global Middlewares
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.Middleware(log))
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.MaxBodySize(5 * 1024 * 1024)) // 5 MB
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		AllowCredentials: true,
	}))

	// Global IP Rate Limiter: 60 req/min
	r.Use(limiter.Middleware(
		ratelimiter.Limit{Rate: 60, Burst: 60, Period: time.Minute, FailureMode: ratelimiter.FailOpen},
		ratelimiter.ByIP,
	))

	// Auth Endpoints
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", func(c *gin.Context) {
			user := UserAccount{UserID: "usr_101", Email: "admin@example.com", Role: "editor"}
			if err := auth.Login(c, user); err != nil {
				httpx.AbortInternal(c, err)
				return
			}
			httpx.OK(c, gin.H{"message": "Logged in successfully"})
		})

		authGroup.POST("/refresh", func(c *gin.Context) {
			if err := auth.Refresh(c); err != nil {
				httpx.AbortUnauthorized(c, "Invalid refresh session")
				return
			}
			httpx.OK(c, gin.H{"message": "Token pair refreshed"})
		})

		authGroup.POST("/logout", func(c *gin.Context) {
			_ = auth.Logout(c)
			httpx.OK(c, gin.H{"message": "Logged out successfully"})
		})
	}

	// Protected API Routes
	api := r.Group("/api", auth.Middleware())
	{
		api.GET("/me", func(c *gin.Context) {
			user := auth.MustPayload(c)
			httpx.OK(c, user)
		})

		api.POST("/articles", func(c *gin.Context) {
			req, ok := httpx.BindJSON[CreateArticleRequest](c)
			if !ok {
				return
			}

			article := Article{
				ID:        1,
				Title:     req.Title,
				Content:   req.Content,
				CreatedAt: time.Now(),
			}

			log.Info(c.Request.Context(), "Created article", "article_id", article.ID)
			httpx.Created(c, article)
		})

		api.GET("/articles", func(c *gin.Context) {
			q, ok := httpx.BindQuery[pagination.Query](c)
			if !ok {
				return
			}

			articles := []Article{
				{ID: 1, Title: "First Article", Content: "Hello World", CreatedAt: time.Now()},
				{ID: 2, Title: "Second Article", Content: "Go Core Package", CreatedAt: time.Now()},
			}
			totalCount := 25

			result := pagination.NewResult(articles, totalCount, q)
			httpx.OK(c, result)
		})
	}

	log.Info(nil, "Server starting on port 8080...")
	r.Run(":8080")
}
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
