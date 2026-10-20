package ratelimiter

import (
	"github.com/abdelrahmanAhmed1x/core/internal/testredis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFailurePolicy(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: time.Millisecond})
	defer c.Close()
	for _, mode := range []FailureMode{FailOpen, FailClosed} {
		r := gin.New()
		r.GET("/", New(c).Middleware(Limit{Rate: 1, Burst: 1, Period: time.Minute, FailureMode: mode}, func(*gin.Context) string { return "test" }), func(c *gin.Context) { c.Status(204) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		want := 204
		if mode == FailClosed {
			want = 503
		}
		if w.Code != want {
			t.Fatal(w.Code)
		}
	}
}
func TestDistributedLimit(t *testing.T) {
	c := testredis.Client(t)
	r := gin.New()
	key := time.Now().String()
	r.GET("/", New(c).Middleware(Limit{Rate: 1, Burst: 1, Period: time.Minute}, func(*gin.Context) string { return key }), func(c *gin.Context) { c.Status(204) })
	for _, want := range []int{204, 429} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != want {
			t.Fatal(w.Code)
		}
		if want == 429 && w.Header().Get("Retry-After") == "" {
			t.Fatal("no retry-after")
		}
	}
}
