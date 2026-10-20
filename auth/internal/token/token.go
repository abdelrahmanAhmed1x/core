// Package token contains the shared, fixed-HS256 JWT implementation.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	jwt "github.com/golang-jwt/jwt/v5"
	"time"
)

type Config struct {
	SecretKey string
	Issuer    string
	Audience  string
	TTL       time.Duration
	Leeway    time.Duration
	Transport transport.Config
}
type Claims[T any] struct {
	Payload T      `json:"payload"`
	Type    string `json:"token_type"`
	jwt.RegisteredClaims
	present bool
}

func Normalize(c Config) Config {
	if len(c.SecretKey) < 32 || c.Issuer == "" || c.Audience == "" || c.TTL < time.Second || c.Leeway < 0 || c.Leeway > time.Minute {
		panic("auth: require 32-byte key, issuer, audience, TTL >= 1s, leeway in [0,1m]")
	}
	c.Transport = transport.Normalize(c.Transport)
	return c
}
func Random() (string, error) {
	var b [32]byte
	_, e := rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:]), e
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func ValidOpaque(s string) bool {
	if len(s) != 43 {
		return false
	}
	b, e := base64.RawURLEncoding.DecodeString(s)
	return e == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == s
}
func Issue[T any](c Config, p T, kind string) (string, error) {
	id, e := Random()
	if e != nil {
		return "", e
	}
	now := time.Now()
	claims := Claims[T]{Payload: p, Type: kind, RegisteredClaims: jwt.RegisteredClaims{Issuer: c.Issuer, Audience: jwt.ClaimStrings{c.Audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(c.TTL)), ID: id}}
	s, e := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(c.SecretKey))
	if len(s) > 16384 {
		return "", transport.ErrInvalid
	}
	return s, e
}
func Verify[T any](c Config, s, kind string) (T, error) {
	var z T
	if len(s) > 16384 {
		return z, transport.ErrInvalid
	}
	claims := new(Claims[T])
	t, e := jwt.ParseWithClaims(s, claims, func(*jwt.Token) (any, error) { return []byte(c.SecretKey), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(c.Issuer), jwt.WithAudience(c.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(c.Leeway))
	if e != nil || !t.Valid || !claims.present || claims.Type != kind || claims.IssuedAt == nil || claims.ID == "" || claims.ExpiresAt == nil || !claims.ExpiresAt.After(claims.IssuedAt.Time) {
		return z, transport.ErrInvalid
	}
	return claims.Payload, nil
}

func (c *Claims[T]) UnmarshalJSON(b []byte) error {
	var wire struct {
		Payload json.RawMessage `json:"payload"`
		Type    string          `json:"token_type"`
		jwt.RegisteredClaims
	}
	if e := json.Unmarshal(b, &wire); e != nil {
		return e
	}
	c.RegisteredClaims = wire.RegisteredClaims
	c.Type = wire.Type
	c.present = len(wire.Payload) > 0
	if !c.present {
		return nil
	}
	return json.Unmarshal(wire.Payload, &c.Payload)
}
