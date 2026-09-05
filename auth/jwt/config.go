package jwt

import (
	"net/http"
	"time"
)

// Config contains application-level JWT and cookie configuration.
//
// Redis is intentionally not part of Config. The Redis client is supplied
// directly to New because it is an infrastructure dependency.
type Config struct {
	SecretKey  string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	CookieDomain   string
	CookiePath     string
	CookieSecure   bool
	CookieSameSite http.SameSite

	// RefreshTokenPath controls where the browser sends the refresh cookie.
	//
	// It defaults to "/".
	//
	// If refresh and logout routes share a prefix such as:
	//
	//   /auth/refresh
	//   /auth/logout
	//
	// then "/auth" is a good value.
	RefreshTokenPath string
}


func applyDefaults(cfg *Config) {
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = defaultAccessTTL
	}

	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = defaultRefreshTTL
	}

	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}

	if cfg.RefreshTokenPath == "" {
		cfg.RefreshTokenPath = "/"
	}

	if cfg.CookieSameSite == 0 {
		cfg.CookieSameSite = http.SameSiteLaxMode
	}
}

func validateConfig(cfg Config) {
	if cfg.SecretKey == "" {
		panic("jwt: secret key cannot be empty")
	}

	// HS256 should use at least a 256-bit key.
	if len([]byte(cfg.SecretKey)) < 32 {
		panic("jwt: secret key must be at least 32 bytes for HS256")
	}

	if cfg.Issuer == "" {
		panic("jwt: issuer cannot be empty")
	}

	if cfg.AccessTTL <= 0 {
		panic("jwt: access TTL must be greater than zero")
	}

	if cfg.RefreshTTL <= 0 {
		panic("jwt: refresh TTL must be greater than zero")
	}

	if cfg.CookiePath[0] != '/' {
		panic("jwt: cookie path must begin with '/'")
	}

	if cfg.RefreshTokenPath[0] != '/' {
		panic("jwt: refresh token path must begin with '/'")
	}
}