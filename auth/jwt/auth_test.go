package jwt

import (
	"context"
	"errors"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/abdelrahmanAhmed1x/core/internal/testredis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func hybrid(t testing.TB) *manager[int] {
	return New[int](testredis.Client(t), Config{Issuer: fmt.Sprintf("test-%d", time.Now().UnixNano()), SecretKey: strings.Repeat("s", 32), CookieSecure: true, RefreshTTL: time.Minute, ReplayGrace: 10 * time.Millisecond}).(*manager[int])
}
func request(token string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "https://app/auth/refresh", nil)
	if token != "" {
		c.Request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: token})
	}
	return c
}
func cookie(c *gin.Context, name string) string {
	for _, h := range c.Writer.Header().Values("Set-Cookie") {
		r := http.Response{Header: http.Header{"Set-Cookie": []string{h}}}
		for _, v := range r.Cookies() {
			if v.Name == name {
				return v.Value
			}
		}
	}
	return ""
}
func TestRotateReplayLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := hybrid(t)
	c := request("")
	if e := m.Login(c, 42); e != nil {
		t.Fatal(e)
	}
	old := cookie(c, refreshCookieName)
	c = request(old)
	if e := m.Refresh(c); e != nil {
		t.Fatal(e)
	}
	next := cookie(c, refreshCookieName)
	if old == next || next == "" {
		t.Fatal("no rotation")
	}
	if e := m.Refresh(request(old)); !errors.Is(e, ErrInvalidRefreshToken) {
		t.Fatal(e)
	}
	time.Sleep(20 * time.Millisecond)
	if e := m.Refresh(request(old)); !errors.Is(e, ErrInvalidRefreshToken) {
		t.Fatal(e)
	}
	if e := m.Refresh(request(next)); !errors.Is(e, ErrInvalidRefreshToken) {
		t.Fatal("replay did not revoke", e)
	}
	c = request("")
	if e := m.Login(c, 7); e != nil {
		t.Fatal(e)
	}
	old = cookie(c, refreshCookieName)
	access := cookie(c, accessCookieName)
	if e := m.Logout(request(old)); e != nil {
		t.Fatal(e)
	}
	if e := m.Refresh(request(old)); !errors.Is(e, ErrInvalidRefreshToken) {
		t.Fatal("logout bypass", e)
	}
	if v, e := m.parseAccessToken(access); e != nil || v != 7 {
		t.Fatal("access JWT semantics changed", v, e)
	}
}
func TestConcurrentRotation(t *testing.T) {
	m := hybrid(t)
	c := request("")
	if e := m.Login(c, 1); e != nil {
		t.Fatal(e)
	}
	old := cookie(c, refreshCookieName)
	var success atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			e := m.Refresh(request(old))
			if e == nil {
				success.Add(1)
			} else if !errors.Is(e, ErrInvalidRefreshToken) {
				t.Error(e)
			}
		})
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal(success.Load())
	}
}
func TestRotationCollisionAndCancellation(t *testing.T) {
	m := hybrid(t)
	ctx := context.Background()
	raw, _ := generateRefreshToken()
	family, _ := generateRefreshToken()
	old := family + "." + raw
	if e := m.storeRefresh(ctx, old, 1); e != nil {
		t.Fatal(e)
	}
	keys := m.familyKeys(old)
	next := family + ".collision"
	nk := m.familyKeys(next)
	if e := m.redis.Set(ctx, nk[0], "existing", time.Minute).Err(); e != nil {
		t.Fatal(e)
	}
	n, e := familyRotate.Run(ctx, m.redis, []string{keys[0], nk[0], keys[1], nk[1]}, "1", hashRefreshToken(old), hashRefreshToken(next), 60000, 1).Int()
	if e != nil || n != 0 {
		t.Fatal(n, e)
	}
	if s := m.redis.Get(ctx, keys[0]).Val(); s != "1" {
		t.Fatal("old token consumed")
	}
	c := request(old)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	c.Request = c.Request.WithContext(canceled)
	if e = m.Refresh(c); e == nil {
		t.Fatal("canceled refresh succeeded")
	}
	if s := m.redis.Get(ctx, keys[0]).Val(); s != "1" {
		t.Fatal("canceled operation consumed token")
	}
}
func TestClaimsCompatibility(t *testing.T) {
	m := &manager[int]{cfg: Config{SecretKey: strings.Repeat("s", 32), Issuer: "i", Audience: "api", AccessTTL: time.Minute}}
	s, e := m.signAccess(4)
	if e != nil {
		t.Fatal(e)
	}
	if v, e := m.parseAccessToken(s); e != nil || v != 4 {
		t.Fatal(v, e)
	}
	m.cfg.Audience = "other"
	if _, e = m.parseAccessToken(s); e == nil {
		t.Fatal("audience ignored")
	}
}
func BenchmarkRefresh(b *testing.B) {
	gin.SetMode(gin.TestMode)
	m := hybrid(b)
	c := request("")
	if e := m.Login(c, 1); e != nil {
		b.Fatal(e)
	}
	s := cookie(c, refreshCookieName)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c = request(s)
		if e := m.Refresh(c); e != nil {
			b.Fatal(e)
		}
		s = cookie(c, refreshCookieName)
	}
}

func TestInjectedRotationFailure(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			m := hybrid(t)
			c := request("")
			if e := m.Login(c, 1); e != nil {
				t.Fatal(e)
			}
			old := cookie(c, refreshCookieName)
			if _, e := familyRotate.Load(context.Background(), m.redis).Result(); e != nil {
				t.Fatal(e)
			}
			boom := errors.New("injected connection loss")
			m.redis.AddHook(testredis.Hook{Process: func(ctx context.Context, cmd redis.Cmder, next redis.ProcessHook) error {
				a := cmd.Args()
				target := (cmd.Name() == "evalsha" && a[1] == familyRotate.Hash())
				if !target {
					return next(ctx, cmd)
				}
				if after {
					if e := next(ctx, cmd); e != nil {
						return e
					}
				}
				return boom
			}})
			c = request(old)
			if e := m.Refresh(c); !errors.Is(e, boom) {
				t.Fatal(e)
			}
			if cookie(c, refreshCookieName) != "" {
				t.Fatal("delivered after ambiguous failure")
			}
			data, e := m.redis.Get(context.Background(), m.familyKeys(old)[0]).Result()
			if e != nil {
				t.Fatal(e)
			}
			if after != strings.HasPrefix(data, "USED:") {
				t.Fatalf("unexpected state after=%v data=%s", after, data)
			}
		})
	}
}

func TestMissingFamilyAndExpiry(t *testing.T) {
	m := hybrid(t)
	ctx := context.Background()
	for _, expired := range []bool{false, true} {
		c := request("")
		if e := m.Login(c, 1); e != nil {
			t.Fatal(e)
		}
		s := cookie(c, refreshCookieName)
		if expired {
			if e := m.redis.PExpire(ctx, m.familyKeys(s)[0], time.Millisecond).Err(); e != nil {
				t.Fatal(e)
			}
			time.Sleep(5 * time.Millisecond)
		} else {
			if e := m.redis.Del(ctx, m.familyKeys(s)[1]).Err(); e != nil {
				t.Fatal(e)
			}
		}
		if e := m.Refresh(request(s)); !errors.Is(e, ErrInvalidRefreshToken) {
			t.Fatal("lost state accepted", e)
		}
	}
}
func TestLegacyRotation(t *testing.T) {
	m := hybrid(t)
	s, e := generateRefreshToken()
	if e != nil {
		t.Fatal(e)
	}
	if e = m.redis.Set(context.Background(), m.refreshKey(s), "42", time.Minute).Err(); e != nil {
		t.Fatal(e)
	}
	c := request(s)
	if e = m.Refresh(c); e != nil {
		t.Fatal(e)
	}
	next := cookie(c, refreshCookieName)
	if !strings.Contains(next, ".") {
		t.Fatal("legacy not migrated")
	}
	if e = m.Refresh(request(next)); e != nil {
		t.Fatal(e)
	}
	if e = m.Refresh(request(s)); !errors.Is(e, ErrInvalidRefreshToken) {
		t.Fatal(e)
	}
}

func TestHybridMiddleware(t *testing.T) {
	m := &manager[int]{cfg: Config{SecretKey: strings.Repeat("s", 32), Issuer: "i", AccessTTL: time.Minute}}
	s, e := m.signAccess(42)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		optional   bool
		credential string
		conflict   bool
		want       int
	}{{false, "", false, 401}, {false, s, false, 204}, {true, "bad", false, 204}, {true, s, true, 401}} {
		r := gin.New()
		mw := m.Middleware()
		if tc.optional {
			mw = m.OptionalMiddleware()
		}
		r.GET("/", mw, func(c *gin.Context) {
			if tc.credential == s && !tc.conflict {
				if m.MustPayload(c) != 42 {
					t.Error("wrong payload")
				}
				if v, ok := transport.Payload[int](c); !ok || v != 42 {
					t.Error("shared context absent")
				}
			}
			c.Status(204)
		})
		req := httptest.NewRequest("GET", "/", nil)
		if tc.credential != "" {
			req.Header.Set("Authorization", "Bearer "+tc.credential)
		}
		if tc.conflict {
			req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "other"})
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatal(w.Code, tc)
		}
	}
}
