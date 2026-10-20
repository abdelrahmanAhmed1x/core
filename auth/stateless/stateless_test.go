package stateless

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPair(t *testing.T) {
	m := New[int](Config{Issuer: "issuer", AccessAudience: "api", RefreshAudience: "refresh", AccessKey: strings.Repeat("a", 32), RefreshKey: strings.Repeat("b", 32), AccessTTL: time.Minute, RefreshTTL: time.Hour})
	p, e := m.Issue(7)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.VerifyAccess(p.Refresh); e == nil {
		t.Fatal("refresh accepted as access")
	}
	if _, e = m.VerifyRefresh(p.Access); e == nil {
		t.Fatal("access accepted as refresh")
	}
	for range 2 {
		next, e := m.Refresh(p.Refresh)
		if e != nil || next.Refresh == p.Refresh {
			t.Fatal(next, e)
		}
	}
	if v, e := m.VerifyAccess(p.Access); e != nil || v != 7 {
		t.Fatal(v, e)
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := New[int](Config{Issuer: "i", AccessAudience: "api", RefreshAudience: "refresh", AccessKey: strings.Repeat("a", 32), RefreshKey: strings.Repeat("b", 32), AccessTTL: time.Minute, RefreshTTL: time.Hour})
	p, e := m.Issue(42)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		optional   bool
		credential string
		want       int
	}{{false, "", 401}, {true, "", 204}, {true, p.Refresh, 401}, {false, p.Access, 204}} {
		r := gin.New()
		mw := m.Middleware()
		if tc.optional {
			mw = m.OptionalMiddleware()
		}
		r.GET("/", mw, func(c *gin.Context) {
			if tc.credential == p.Access {
				if v, ok := m.Payload(c); !ok || v != 42 {
					t.Error(v, ok)
				}
			}
			c.Status(204)
		})
		req := httptest.NewRequest("GET", "/", nil)
		if tc.credential != "" {
			req.Header.Set("Authorization", "Bearer "+tc.credential)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatal(w.Code)
		}
	}
}
