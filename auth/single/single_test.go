package single

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSingle(t *testing.T) {
	m := New[int](Config{SecretKey: strings.Repeat("s", 32), Issuer: "test", Audience: "api", TTL: time.Minute})
	s, e := m.Issue(7)
	if e != nil {
		t.Fatal(e)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/", func(c *gin.Context) {
		p, ok := m.Payload(c)
		if !ok || p != 7 {
			t.Error(p, ok)
		}
		c.Status(204)
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	if _, e := m.Verify(s + "x"); e == nil {
		t.Fatal("modified signature accepted")
	}
}
