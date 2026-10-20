package transport

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtract(t *testing.T) {
	cfg := Normalize(Config{AllowedOrigins: []string{"https://app.example"}})
	for _, tc := range []struct {
		name, method, cookie, header, origin string
		want                                 error
	}{
		{"missing", "GET", "", "", "", ErrMissing}, {"bearer", "GET", "", "Bearer abc", "", nil}, {"cookie", "GET", "abc", "", "", nil}, {"conflict", "GET", "a", "Bearer b", "", ErrConflict}, {"bad scheme", "GET", "", "Basic x", "", ErrInvalid}, {"csrf", "POST", "a", "", "", ErrCSRF}, {"origin", "POST", "a", "", "https://app.example", nil}, {"same credentials csrf", "POST", "a", "Bearer a", "", ErrCSRF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "https://app.example", nil)
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: tc.cookie})
			}
			r.Header.Set("Authorization", tc.header)
			r.Header.Set("Origin", tc.origin)
			_, e := Extract(r, cfg)
			if !errors.Is(e, tc.want) {
				t.Fatalf("got %v want %v", e, tc.want)
			}
		})
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: "x"})
	r.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: "x"})
	if _, e := Extract(r, cfg); e != ErrConflict {
		t.Fatal(e)
	}
}
func TestCookies(t *testing.T) {
	cfg := Normalize(Config{})
	for _, ttl := range []time.Duration{time.Minute, 0} {
		w := httptest.NewRecorder()
		SetCookie(w, cfg, "x", ttl)
		cs := w.Result().Cookies()
		if len(cs) != 1 {
			t.Fatal(cs)
		}
		c := cs[0]
		if !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" || c.SameSite != http.SameSiteLaxMode {
			t.Fatal(c)
		}
		if ttl == 0 && c.MaxAge != -1 {
			t.Fatal(c)
		}
	}
	defer func() {
		if recover() == nil {
			t.Fatal("unsafe host cookie accepted")
		}
	}()
	Normalize(Config{InsecureDevelopment: true})
}
func TestOptionalClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := Normalize(Config{})
	for _, tc := range []struct {
		credential string
		err        error
		want       int
	}{{"", nil, 204}, {"Bearer bad", ErrInvalid, 401}, {"Bearer good", errors.New("backend"), 503}, {"Bearer good", nil, 204}} {
		r := gin.New()
		r.Use(Middleware(cfg, true, func(context.Context, string) (string, error) { return "principal", tc.err }))
		r.GET("/", func(c *gin.Context) { c.Status(204) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", tc.credential)
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
}
func FuzzExtract(f *testing.F) {
	f.Add("Bearer abc", "x")
	f.Fuzz(func(t *testing.T, h, c string) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Authorization", h)
		r.Header.Set("Cookie", c)
		Extract(r, Config{CookieName: "session"})
	})
}
