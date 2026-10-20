// Package transport provides explicit cookie/bearer authentication adapters.
package transport

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	ErrMissing  = errors.New("auth: credential missing")
	ErrInvalid  = errors.New("auth: invalid credential")
	ErrConflict = errors.New("auth: conflicting credentials")
	ErrCSRF     = errors.New("auth: untrusted cookie request origin")
)

const payloadKey = "core.auth.payload"

// Config controls cookie delivery and Origin-based CSRF checks. Zero values use
// a Secure, HttpOnly, SameSite=Lax, __Host-session cookie on path /.
// InsecureDevelopment is exclusively for local HTTP development.
type Config struct {
	CookieName          string
	Domain              string
	Path                string
	SameSite            http.SameSite
	InsecureDevelopment bool
	AllowedOrigins      []string
}

func Normalize(c Config) Config {
	if c.CookieName == "" {
		c.CookieName = "__Host-session"
	}
	if c.Path == "" {
		c.Path = "/"
	}
	if c.SameSite == 0 {
		c.SameSite = http.SameSiteLaxMode
	}
	c.AllowedOrigins = append([]string(nil), c.AllowedOrigins...)
	for _, o := range c.AllowedOrigins {
		if !ValidateOrigin(o) {
			panic("auth: invalid allowed origin")
		}
	}
	if err := (&http.Cookie{Name: c.CookieName, Path: c.Path, Domain: c.Domain}).Valid(); err != nil {
		panic("auth: invalid cookie configuration")
	}
	if strings.ContainsAny(c.CookieName, " ;,\r\n\t=") || c.Path[0] != '/' || strings.ContainsAny(c.Path+c.Domain, ";\r\n") {
		panic("auth: invalid cookie configuration")
	}
	if strings.HasPrefix(c.CookieName, "__Host-") && (c.Domain != "" || c.Path != "/" || c.InsecureDevelopment) {
		panic("auth: __Host- cookies require Secure, Path=/ and no Domain")
	}
	if strings.HasPrefix(c.CookieName, "__Secure-") && c.InsecureDevelopment {
		panic("auth: __Secure- cookies require HTTPS")
	}
	if c.SameSite == http.SameSiteNoneMode && c.InsecureDevelopment {
		panic("auth: SameSite=None requires Secure")
	}
	return c
}

// Extract rejects duplicate headers/cookies and conflicting transports. Cookie
// authentication on unsafe methods requires an explicitly allowed Origin.
func Extract(r *http.Request, c Config) (string, error) {
	if len(r.Header.Values("Origin")) > 1 {
		return "", ErrCSRF
	}
	var cookie, bearer string
	n := 0
	for _, v := range r.Cookies() {
		if v.Name == c.CookieName {
			n++
			cookie = v.Value
		}
	}
	if n > 1 || len(r.Header.Values("Authorization")) > 1 {
		return "", ErrConflict
	}
	if h := r.Header.Get("Authorization"); h != "" {
		p := strings.Fields(h)
		if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
			return "", ErrInvalid
		}
		bearer = p[1]
	}
	if cookie != "" && bearer != "" && cookie != bearer {
		return "", ErrConflict
	}
	if cookie != "" && r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
		origin := r.Header.Get("Origin")
		ok := false
		for _, v := range c.AllowedOrigins {
			if v == origin && origin != "" {
				ok = true
			}
		}
		if !ok {
			return "", ErrCSRF
		}
	}
	if bearer != "" {
		return bearer, nil
	}
	if cookie != "" {
		return cookie, nil
	}
	return "", ErrMissing
}

// ValidateOrigin validates origins supplied by the application at startup.
func ValidateOrigin(s string) bool {
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.Path == "" && u.RawQuery == "" && u.User == nil && u.Fragment == ""
}
func SetCookie(w http.ResponseWriter, c Config, value string, ttl time.Duration) {
	age := int((ttl + time.Second - 1) / time.Second)
	expires := time.Now().Add(ttl)
	if ttl <= 0 {
		age = -1
		expires = time.Unix(1, 0)
	}
	http.SetCookie(w, &http.Cookie{Name: c.CookieName, Value: value, Path: c.Path, Domain: c.Domain, Secure: !c.InsecureDevelopment, HttpOnly: true, SameSite: c.SameSite, MaxAge: age, Expires: expires})
}
func Payload[T any](c *gin.Context) (T, bool) {
	var z T
	v, ok := c.Get(payloadKey)
	if !ok {
		return z, false
	}
	p, ok := v.(T)
	return p, ok
}
func StorePayload[T any](c *gin.Context, p T) { c.Set(payloadKey, p) }

// Middleware treats only absent credentials as guests in optional mode. Supplied
// invalid credentials fail with 401; backend failures fail with 503.
func Middleware[T any](cfg Config, optional bool, verify func(context.Context, string) (T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, e := Extract(c.Request, cfg)
		if optional && errors.Is(e, ErrMissing) {
			c.Next()
			return
		}
		if e != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		p, e := verify(c.Request.Context(), raw)
		if e != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(e, ErrInvalid) {
				status = http.StatusUnauthorized
			}
			c.AbortWithStatus(status)
			return
		}
		StorePayload(c, p)
		c.Next()
	}
}
