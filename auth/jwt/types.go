package jwt

import (
	"errors"
	"time"
		golangjwt "github.com/golang-jwt/jwt/v5"
)

const (
	accessCookieName  = "access_token"
	refreshCookieName = "refresh_token"

	payloadContextKey = "jwt_payload"

	defaultAccessTTL  = 15 * time.Minute
	defaultRefreshTTL = 7 * 24 * time.Hour
)

var (
	ErrMissingAccessToken  = errors.New("jwt: missing access token")
	ErrInvalidAccessToken  = errors.New("jwt: invalid access token")
	ErrMissingRefreshToken = errors.New("jwt: missing refresh token")
	ErrInvalidRefreshToken = errors.New("jwt: invalid refresh token")
)

type claims[T any] struct {
	Payload T `json:"payload"`
	golangjwt.RegisteredClaims
}
