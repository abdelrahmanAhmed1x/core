package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		origins        []string
		origin, method string
		want           int
		allow          string
	}{{[]string{"*"}, "https://a", "GET", 204, "*"}, {[]string{"https://a"}, "https://a", "POST", 204, "https://a"}, {[]string{"https://a"}, "https://b", "POST", 403, ""}, {[]string{"https://a"}, "https://a", "DELETE", 403, "https://a"}} {
		r := gin.New()
		r.Use(CORS(CORSConfig{AllowedOrigins: tc.origins, AllowedMethods: []string{"GET", "POST"}}))
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Access-Control-Request-Method", tc.method)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want || w.Header().Get("Access-Control-Allow-Origin") != tc.allow {
			t.Fatal(w.Code, w.Header())
		}
	}
}
func TestTimeout(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(1))
	r.GET("/", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); !ok {
			t.Error("no deadline")
		}
		c.Status(204)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
func BenchmarkSecurityHeaders(b *testing.B) {
	gin.SetMode(gin.TestMode)
	m := SecurityHeaders()
	b.ReportAllocs()
	for b.Loop() {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		m(c)
	}
}
