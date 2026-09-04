# Core

`core` is a lightweight, production-grade Go toolkit designed for building robust, scalable web applications and microservices using [Gin](https://github.com/gin-gonic/gin). It provides strongly-typed generics, context-aware logging, standardized HTTP response envelopes, pagination utilities, and dual-mode authentication (JWT cookies and Opaque token session management).

---

## Table of Contents

- [Installation](#installation)
- [Package Overview](#package-overview)
- [Packages & Usage](#packages--usage)
  - [1. `httpx` — Standardized HTTP Responses & Binding](#1-httpx--standardized-http-responses--binding)
  - [2. `auth/jwt` — Cookie-Based JWT Authentication](#2-authjwt--cookie-based-jwt-authentication)
  - [3. `auth/opaqueauth` — Stateful Opaque Session Auth](#3-authopaqueauth--stateful-opaque-session-auth)
  - [4. `logger` — Context-Aware Structured Logging](#4-logger--context-aware-structured-logging)
  - [5. `pagination` — Generic Request & Response Pagination](#5-pagination--generic-request--response-pagination)
- [Complete Full-Server Example](#complete-full-server-example)
- [License](#license)

---

## Installation

```bash
go get github.com/abdelrahmanAhmed1x/core
```

---

## Package Overview

| Package | Import Path | Description |
| :--- | :--- | :--- |
| **`httpx`** | `github.com/abdelrahmanAhmed1x/core/httpx` | Standardized JSON response envelopes, generic request binders (JSON, URI, Query), and HTTP abort helpers. |
| **`auth/jwt`** | `github.com/abdelrahmanAhmed1x/core/auth/jwt` | Cookie-based JWT authentication manager with generic strongly-typed payloads and Gin middleware. |
| **`auth/opaqueauth`** | `github.com/abdelrahmanAhmed1x/core/auth/opaqueauth` | Stateful opaque token authentication with SHA-256 hashing, token rotation, and flexible storage interfaces. |
| **`logger`** | `github.com/abdelrahmanAhmed1x/core/logger` | `slog`-backed structured logging with request tracing, automatic sensitive key masking, and Gin middleware. |
| **`pagination`** | `github.com/abdelrahmanAhmed1x/core/pagination` | Type-safe pagination query binding (`page`, `limit`), DB offset calculation, and paginated response builders. |

---

## Packages & Usage

### 1. `httpx` — Standardized HTTP Responses & Binding

The `httpx` package ensures a consistent API response structure across your application, eliminating boilerplate code for request binding and error handling.

#### Standardized JSON Envelope Format

**Success Response:**
```json
{
  "success": true,
  "data": { ... }
}
```

**Error Response:**
```json
{
  "success": false,
  "error": {
    "code": "BAD_REQUEST",
    "message": "Invalid request payload",
    "details": {
      "Email": "required"
    }
  }
}
```

#### Code Example

```go
package main

import (
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
)

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type UserParam struct {
	ID string `uri:"id" binding:"required"`
}

func createUserHandler(c *gin.Context) {
	// 1. Bind and validate JSON body (automatically calls AbortBadRequest on error)
	req, ok := httpx.BindJSON[CreateUserRequest](c)
	if !ok {
		return // Request was automatically aborted
	}

	// 2. Return 201 Created with standardized envelope
	httpx.Created(c, gin.H{
		"id":    "user_123",
		"name":  req.Name,
		"email": req.Email,
	})
}

func getUserHandler(c *gin.Context) {
	// Bind path parameters (e.g. /users/:id)
	params, ok := httpx.BindURI[UserParam](c)
	if !ok {
		return
	}

	if params.ID == "404" {
		httpx.AbortNotFound(c, "User not found")
		return
	}

	httpx.OK(c, gin.H{"id": params.ID, "name": "John Doe"})
}

func main() {
	r := gin.Default()
	r.POST("/users", createUserHandler)
	r.GET("/users/:id", getUserHandler)
	r.Run(":8080")
}
```

#### API Reference

- **Responses**:
  - `httpx.OK[T](c *gin.Context, data T)` — Sends `200 OK`
  - `httpx.Created[T](c *gin.Context, data T)` — Sends `201 Created`
  - `httpx.NoContent(c *gin.Context)` — Sends `204 No Content`
- **Request Binders**:
  - `httpx.BindJSON[T](c *gin.Context) (T, bool)` — Binds request JSON to `T`
  - `httpx.BindURI[T](c *gin.Context) (T, bool)` — Binds route URI parameters to `T`
  - `httpx.BindQuery[T](c *gin.Context) (T, bool)` — Binds URL query strings to `T`
- **Error Aborts**:
  - `httpx.AbortBadRequest(c, message, details)` — `400 Bad Request`
  - `httpx.AbortUnauthorized(c, message)` — `401 Unauthorized`
  - `httpx.AbortForbidden(c, message)` — `403 Forbidden`
  - `httpx.AbortNotFound(c, message)` — `404 Not Found`
  - `httpx.AbortInternal(c, err)` — `500 Internal Server Error` (logs cause safely)

---

### 2. `auth/jwt` — Cookie-Based JWT Authentication

The `auth/jwt` package manages JWT-based authentication using HttpOnly cookies for secure access and refresh token lifecycle operations. It supports custom, strongly-typed payloads `T`.

#### Code Example

```go
package main

import (
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
)

// UserSession defines the strongly-typed payload embedded in tokens
type UserSession struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func main() {
	r := gin.Default()

	// Initialize JWT Authenticator with custom payload
	authManager := jwt.New[UserSession](jwt.Config{
		SecretKey:        "super-secret-key-change-in-production",
		Issuer:           "my-app",
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       7 * 24 * time.Hour,
		CookieSecure:     false, // Set true in production (HTTPS)
		RefreshTokenPath: "/auth/refresh",
	})

	// Auth routes
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", func(c *gin.Context) {
			sessionPayload := UserSession{UserID: "user_42", Role: "admin"}

			// Login issues access & refresh tokens in HttpOnly cookies
			if err := authManager.Login(c, sessionPayload); err != nil {
				httpx.AbortInternal(c, err)
				return
			}
			httpx.OK(c, gin.H{"message": "Logged in successfully"})
		})

		authGroup.POST("/refresh", func(c *gin.Context) {
			// Refreshes tokens using valid refresh cookie
			if err := authManager.Refresh(c); err != nil {
				httpx.AbortUnauthorized(c, "Invalid or expired refresh token")
				return
			}
			httpx.OK(c, gin.H{"message": "Token refreshed"})
		})

		authGroup.POST("/logout", func(c *gin.Context) {
			// Clears auth cookies
			authManager.Logout(c)
			httpx.OK(c, gin.H{"message": "Logged out successfully"})
		})
	}

	// Protected routes
	protected := r.Group("/api", authManager.Middleware())
	{
		protected.GET("/profile", func(c *gin.Context) {
			// Extract payload from context
			payload := jwt.MustExtractPayload[UserSession](c)
			httpx.OK(c, gin.H{
				"user_id": payload.UserID,
				"role":    payload.Role,
			})
		})
	}

	r.Run(":8080")
}
```

#### API Reference

- `jwt.New[T](cfg jwt.Config) jwt.Authenticator[T]` — Instantiates a JWT manager.
- `Authenticator[T]` Interface:
  - `Middleware() gin.HandlerFunc` — Gin middleware protecting routes.
  - `Login(c *gin.Context, payload T) error` — Issues access and refresh cookies.
  - `Refresh(c *gin.Context) error` — Validates refresh cookie and re-issues token pair.
  - `Logout(c *gin.Context)` — Clears cookies.
- Payload Extraction:
  - `jwt.ExtractPayload[T](c *gin.Context) (T, error)` — Safely retrieves token payload from `gin.Context`.
  - `jwt.MustExtractPayload[T](c *gin.Context) T` — Retrieves token payload or panics if unauthenticated.

---

### 3. `auth/opaqueauth` — Stateful Opaque Session Auth

For cases where token revocation, session listing, or server-side state is required, `auth/opaqueauth` provides an opaque session manager. Tokens are stored exclusively as SHA-256 hashes on the backend (Redis, Postgres, Memory), ensuring raw token strings never touch your database.

#### Code Example

```go
package main

import (
	"context"
	"sync"
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/opaqueauth"
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
)

type SessionPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// MemoryStore implements opaqueauth.Store[T] interface
type MemoryStore[T any] struct {
	mu           sync.RWMutex
	accessMap    map[string]opaqueauth.Session[T]
	refreshMap   map[string]opaqueauth.Session[T]
}

func NewMemoryStore[T any]() *MemoryStore[T] {
	return &MemoryStore[T]{
		accessMap:  make(map[string]opaqueauth.Session[T]),
		refreshMap: make(map[string]opaqueauth.Session[T]),
	}
}

func (s *MemoryStore[T]) Save(ctx context.Context, session opaqueauth.Session[T]) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessMap[session.AccessTokenHash] = session
	s.refreshMap[session.RefreshTokenHash] = session
	return nil
}

func (s *MemoryStore[T]) GetByAccessHash(ctx context.Context, hash string) (opaqueauth.Session[T], error) {
	s.mu.RLock()
	defer s.mu.RWMutex.RUnlock()
	sess, exists := s.accessMap[hash]
	if !exists {
		return opaqueauth.Session[T]{}, opaqueauth.ErrInvalidToken
	}
	return sess, nil
}

func (s *MemoryStore[T]) GetByRefreshHash(ctx context.Context, hash string) (opaqueauth.Session[T], error) {
	s.mu.RLock()
	defer s.mu.RWMutex.RUnlock()
	sess, exists := s.refreshMap[hash]
	if !exists {
		return opaqueauth.Session[T]{}, opaqueauth.ErrInvalidToken
	}
	return sess, nil
}

func (s *MemoryStore[T]) RevokeByAccessHash(ctx context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accessMap, hash)
	return nil
}

func (s *MemoryStore[T]) RevokeByRefreshHash(ctx context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshMap, hash)
	return nil
}

func (s *MemoryStore[T]) RevokeAllForUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.accessMap {
		if v.UserID == userID {
			delete(s.accessMap, k)
		}
	}
	for k, v := range s.refreshMap {
		if v.UserID == userID {
			delete(s.refreshMap, k)
		}
	}
	return nil
}

func main() {
	store := NewMemoryStore[SessionPayload]()
	mgr := opaqueauth.New[SessionPayload](store, opaqueauth.Config{
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	})

	r := gin.Default()

	// Login endpoint
	r.POST("/login", func(c *gin.Context) {
		pair, err := mgr.Issue(c.Request.Context(), "user_99", SessionPayload{
			UserID: "user_99",
			Email:  "user@example.com",
		})
		if err != nil {
			httpx.AbortInternal(c, err)
			return
		}
		httpx.OK(c, pair)
	})

	// Protected routes using opaque session middleware
	protected := r.Group("/dashboard", opaqueauth.Middleware(mgr, "access_token"))
	{
		protected.GET("/me", func(c *gin.Context) {
			payload := opaqueauth.MustGetPayload[SessionPayload](c)
			httpx.OK(c, payload)
		})
	}

	r.Run(":8080")
}
```

---

### 4. `logger` — Context-Aware Structured Logging

The `logger` package wraps Go's `log/slog` stdlib package, offering automatic context tracing (`X-Request-ID`), sensitive field redaction, and HTTP access logging middleware.

#### Features

- **Context Extraction**: Automatically logs `request_id` and `user_id` stored in `context.Context`.
- **Sensitive Data Masking**: Redacts keys like `password`, `token`, `secret`, `authorization`, `cookie`, etc.
- **Gin Middleware**: Tracing & response status logging with latency metrics.

#### Code Example

```go
package main

import (
	"context"

	"github.com/abdelrahmanAhmed1x/core/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize logger with sensitive data masking enabled
	l := logger.New(logger.Config{
		Level:          "debug", // debug, info, warn, error
		Format:         "json",  // json or text
		MaskSensitives: true,   // Automatically masks "password", "token", etc.
	})

	r := gin.New()

	// Attach request-tracing middleware (Injects X-Request-ID into header & context)
	r.Use(logger.Middleware(l))

	r.POST("/login", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Sensitive attributes are automatically redacted in logs
		l.Info(ctx, "User login attempt", 
			"username", "john_doe", 
			"password", "my-secret-pass", // Will be output as "[REDACTED]"
		)

		c.JSON(200, gin.H{"status": "ok"})
	})

	r.Run(":8080")
}
```

---

### 5. `pagination` — Generic Request & Response Pagination

The `pagination` package simplifies handling paginated lists with standard page and limit bounds, DB offset calculation, and JSON metadata generation.

#### Code Example

```go
package main

import (
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/abdelrahmanAhmed1x/core/pagination"
	"github.com/gin-gonic/gin"
)

type Product struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func listProductsHandler(c *gin.Context) {
	// 1. Bind query parameters (?page=2&limit=10) with defaults (page=1, limit=10)
	q, ok := httpx.BindQuery[pagination.Query](c)
	if !ok {
		return
	}

	// Calculate database offset for SQL: SELECT * FROM products LIMIT q.Limit OFFSET q.Offset()
	offset := q.Offset()
	_ = offset

	// Mock data source
	products := []Product{
		{ID: 11, Name: "Keyboard"},
		{ID: 12, Name: "Mouse"},
	}
	totalItems := 45

	// 2. Construct generic paginated result
	result := pagination.NewResult(products, totalItems, q)

	// 3. Return response enveloped with httpx
	httpx.OK(c, result)
}

func main() {
	r := gin.Default()
	r.GET("/products", listProductsHandler)
	r.Run(":8080")
}
```

**JSON Output (`GET /products?page=2&limit=10`):**
```json
{
  "success": true,
  "data": {
    "items": [
      { "id": 11, "name": "Keyboard" },
      { "id": 12, "name": "Mouse" }
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

---

## Complete Full-Server Example

Here is a complete, runnable server demonstrating how all packages in `core` integrate seamlessly together:

```go
package main

import (
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/abdelrahmanAhmed1x/core/logger"
	"github.com/abdelrahmanAhmed1x/core/pagination"
	"github.com/gin-gonic/gin"
)

type Account struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Article struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func main() {
	// 1. Setup Logger
	log := logger.New(logger.Config{
		Level:          "info",
		Format:         "text",
		MaskSensitives: true,
	})

	// 2. Setup JWT Auth
	auth := jwt.New[Account](jwt.Config{
		SecretKey:    "super-secret-key-12345",
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   24 * time.Hour,
		CookieSecure: false,
	})

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.Middleware(log))

	// Auth Handlers
	r.POST("/login", func(c *gin.Context) {
		acc := Account{ID: "usr_100", Email: "admin@example.com"}
		if err := auth.Login(c, acc); err != nil {
			httpx.AbortInternal(c, err)
			return
		}
		httpx.OK(c, gin.H{"message": "Login successful"})
	})

	r.POST("/logout", func(c *gin.Context) {
		auth.Logout(c)
		httpx.OK(c, gin.H{"message": "Logout successful"})
	})

	// Protected API Routes
	api := r.Group("/api", auth.Middleware())
	{
		api.GET("/me", func(c *gin.Context) {
			user := jwt.MustExtractPayload[Account](c)
			httpx.OK(c, user)
		})

		api.GET("/articles", func(c *gin.Context) {
			q, ok := httpx.BindQuery[pagination.Query](c)
			if !ok {
				return
			}

			items := []Article{
				{ID: 1, Title: "Getting Started with Go"},
				{ID: 2, Title: "Building APIs with Gin"},
			}
			result := pagination.NewResult(items, 50, q)
			httpx.OK(c, result)
		})
	}

	log.Info(nil, "Starting server on :8080")
	r.Run(":8080")
}
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
