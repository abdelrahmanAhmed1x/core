package stateful

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/abdelrahmanAhmed1x/core/internal/testredis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func manager(t testing.TB) *Manager[int64] {
	c := testredis.Client(t)
	return New[int64](c, Config{Namespace: fmt.Sprintf("test-%d", time.Now().UnixNano()), AbsoluteTTL: 2 * time.Second, IdleTTL: 500 * time.Millisecond})
}
func TestSessions(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	const large = int64(9007199254740993)
	a, e := m.Create(ctx, "alice", large)
	if e != nil {
		t.Fatal(e)
	}
	b, e := m.Create(ctx, "alice", 2)
	if e != nil {
		t.Fatal(e)
	}
	other, e := m.Create(ctx, "bob", 3)
	if e != nil {
		t.Fatal(e)
	}
	if v, e := m.Verify(ctx, a); e != nil || v != large {
		t.Fatal(v, e)
	}
	if e = m.Revoke(ctx, a); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, a); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if e = m.RevokeAll(ctx, "alice"); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, b); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if v, e := m.Verify(ctx, other); e != nil || v != 3 {
		t.Fatal(v, e)
	}
	fresh, e := m.Create(ctx, "alice", 4)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, fresh); e != nil {
		t.Fatal(e)
	}
}
func TestExpirationRenewCorruption(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	s, e := m.Create(ctx, "u", 1)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(300 * time.Millisecond)
	if _, e = m.Renew(ctx, s); e != nil {
		t.Fatal(e)
	}
	time.Sleep(300 * time.Millisecond)
	if _, e = m.Verify(ctx, s); e != nil {
		t.Fatal(e)
	}
	time.Sleep(300 * time.Millisecond)
	if _, e = m.Verify(ctx, s); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	s, e = m.Create(ctx, "u", 1)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.client.Set(ctx, m.key(s), "{broken", time.Minute).Err(); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, s); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, "bad"); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func TestConcurrentRevoke(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			s, e := m.Create(ctx, "u", int64(i))
			if e != nil {
				t.Error(e)
				return
			}
			m.Renew(ctx, s)
			if e = m.RevokeAll(ctx, "u"); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
	if e := m.RevokeAll(ctx, "u"); e != nil {
		t.Fatal(e)
	}
}
func TestFailClosed(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 10 * time.Millisecond})
	defer c.Close()
	m := New[int](c, Config{Namespace: "failure", AbsoluteTTL: time.Minute, OperationTimeout: 20 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := m.Create(ctx, "u", 1); e == nil {
		t.Fatal("cancellation ignored")
	}
	if _, e := m.Verify(context.Background(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"); e == nil || errors.Is(e, ErrInvalid) {
		t.Fatalf("infrastructure error lost: %v", e)
	}
}
func TestCollision(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	keys := []string{m.prefix + "collision", m.prefix + "epoch"}
	if e := m.client.Set(ctx, keys[0], "existing", time.Second).Err(); e != nil {
		t.Fatal(e)
	}
	n, e := create.Run(ctx, m.client, keys, `{"payload":"1","user":"u"}`, "e", 1000, 0).Int()
	if e != nil || n != 0 {
		t.Fatal(n, e)
	}
	if s := m.client.Get(ctx, keys[0]).Val(); s != "existing" {
		t.Fatal("collision overwrote data")
	}
}
func BenchmarkCreate(b *testing.B) {
	m := manager(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		s, e := m.Create(ctx, "bench", 1)
		if e != nil {
			b.Fatal(e)
		}
		_ = s
	}
}
func BenchmarkVerify(b *testing.B) {
	m := manager(b)
	m.cfg.AbsoluteTTL = time.Hour
	m.cfg.IdleTTL = 0
	s, e := m.Create(context.Background(), "bench", 1)
	if e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, e := m.Verify(context.Background(), s); e != nil {
			b.Fatal(e)
		}
	}
}

func TestLogicalExpirationAndStateLoss(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	s, e := m.Create(ctx, "u", 1)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := m.client.Get(ctx, m.key(s)).Bytes()
	if e != nil {
		t.Fatal(e)
	}
	var r record
	if e = json.Unmarshal(raw, &r); e != nil {
		t.Fatal(e)
	}
	r.Expires = time.Now().Add(-time.Second).UnixMilli()
	raw, e = json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.client.Set(ctx, m.key(s), raw, time.Minute).Err(); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, s); !errors.Is(e, ErrInvalid) {
		t.Fatal("relied only on TTL", e)
	}
	s, e = m.Create(ctx, "u", 2)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.client.Del(ctx, m.prefix+"user:"+token.Hash("u")).Err(); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Verify(ctx, s); !errors.Is(e, ErrInvalid) {
		t.Fatal("missing generation accepted", e)
	}
}
func TestAbsoluteRenewalBound(t *testing.T) {
	m := manager(t)
	m.cfg.AbsoluteTTL = 100 * time.Millisecond
	m.cfg.IdleTTL = time.Second
	s, e := m.Create(context.Background(), "u", 1)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(60 * time.Millisecond)
	if _, e = m.Renew(context.Background(), s); e != nil {
		t.Fatal(e)
	}
	time.Sleep(60 * time.Millisecond)
	if _, e = m.Verify(context.Background(), s); !errors.Is(e, ErrInvalid) {
		t.Fatal("renew extended absolute expiry", e)
	}
}

func TestWebMobileAdapters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := manager(t)
	m.cfg.Transport.AllowedOrigins = []string{"https://app.example"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "https://app.example/login", nil)
	if e := m.Login(c, "u", 42); e != nil {
		t.Fatal(e)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatal(cookies)
	}
	credential := cookies[0].Value
	r := gin.New()
	r.GET("/", m.Middleware(), func(c *gin.Context) {
		v, ok := m.Payload(c)
		if !ok || v != 42 {
			t.Error(v, ok)
		}
		c.Status(204)
	})
	r.GET("/optional", m.OptionalMiddleware(), func(c *gin.Context) { c.Status(204) })
	for _, bearer := range []bool{false, true} {
		req := httptest.NewRequest("GET", "/", nil)
		if bearer {
			req.Header.Set("Authorization", "Bearer "+credential)
		} else {
			req.AddCookie(cookies[0])
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 204 {
			t.Fatal(w.Code)
		}
	}
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "https://app.example/logout", nil)
	c.Request.AddCookie(cookies[0])
	c.Request.Header.Set("Origin", "https://app.example")
	if e := m.Logout(c); e != nil {
		t.Fatal(e)
	}
	if w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("cookie not deleted")
	}
	if _, e := m.Verify(context.Background(), credential); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	m.client.Close()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+credential)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal("store outage accepted", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/optional", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
