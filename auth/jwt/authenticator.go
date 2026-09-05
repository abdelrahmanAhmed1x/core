package jwt

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Authenticator provides access-token authentication and
// Redis-backed opaque refresh-token management.
type Authenticator[T any] interface {
	Middleware() gin.HandlerFunc

	//lenient optional auth:
    //missing token→ guest
    // bad token→ guest
	// no token->guest
	OptionalMiddleware() gin.HandlerFunc

	// Login creates an access JWT and opaque refresh token,
	// stores the refresh session in Redis, and sets both cookies.
	Login(c *gin.Context, payload T) error

	// Refresh atomically consumes the current refresh token and
	// rotates both the access and refresh tokens.
	Refresh(c *gin.Context) error

	// Logout revokes the current refresh token and clears both cookies.
	Logout(c *gin.Context) error

	Payload(c *gin.Context) (T, bool)
	MustPayload(c *gin.Context) T
}

func New[T any](
	client redis.UniversalClient,
	cfg Config,
) Authenticator[T] {
	if client == nil {
		panic("jwt: redis client cannot be nil")
	}

	applyDefaults(&cfg)
	validateConfig(cfg)

	return &manager[T]{
		redis: client,
		cfg:   cfg,
	}
}
