package jwt

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrWrongType    = errors.New("incorrect token type provided")
)

type Config struct {
	SecretKey        string
	Issuer           string
	AccessTTL        time.Duration // Default: 15m
	RefreshTTL       time.Duration // Default: 7d
	CookieDomain     string        // Optional: "example.com"
	CookiePath       string        // Default: "/"
	CookieSecure     bool          // true in production
	CookieSameSite   http.SameSite // Default: http.SameSiteLaxMode
	RefreshTokenPath string        // Default: "/refresh" (scoping refresh cookie)
}

// Only exported interface in the package
type Authenticator[T any] interface {
	Middleware() gin.HandlerFunc           // authenticate middleware
	Login(c *gin.Context, payload T) error // takes the payload creates the tokens and then puts them in cookies
	Refresh(c *gin.Context) error          // uses refresh tokens to re generate new access and refresh
	Logout(c *gin.Context)                 // clears token from cookies
}

type manager[T any] struct {
	cfg Config
}

func New[T any](cfg Config) Authenticator[T] {
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}
	if cfg.RefreshTokenPath == "" {
		cfg.RefreshTokenPath = "/refresh"
	}
	if cfg.CookieSameSite == 0 {
		cfg.CookieSameSite = http.SameSiteLaxMode
	}
	return &manager[T]{cfg: cfg}
}

type customClaims[T any] struct {
	Payload   T      `json:"payload"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func (m *manager[T]) createToken(
	payload T,
	tokenType string,
	ttl time.Duration,
) (string, error) {
	now := time.Now()
	claims := customClaims[T]{
		Payload:   payload,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.cfg.SecretKey))
}

func (m *manager[T]) parseToken(
	tokenStr string, expectedType string,
) (*customClaims[T], error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&customClaims[T]{},
		func(t *jwt.Token) (interface{}, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return []byte(m.cfg.SecretKey), nil
		},
	)

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*customClaims[T])
	if !ok || claims.TokenType != expectedType {
		return nil, ErrWrongType
	}

	return claims, nil
}
