// Package handbook_test contains compile-checked snippets used by the READMEs.
// Authentication helpers accept an already-verified principal; none mount an
// unauthenticated credential-issuance route. Redis examples are compiled only.
package handbook_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	hybrid "github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/auth/single"
	"github.com/abdelrahmanAhmed1x/core/auth/stateful"
	"github.com/abdelrahmanAhmed1x/core/auth/stateless"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/abdelrahmanAhmed1x/core/authorization"
	"github.com/abdelrahmanAhmed1x/core/cache"
	"github.com/abdelrahmanAhmed1x/core/cache/memory"
	rediscache "github.com/abdelrahmanAhmed1x/core/cache/redis"
	"github.com/abdelrahmanAhmed1x/core/cache/tiered"
	"github.com/abdelrahmanAhmed1x/core/health"
	"github.com/abdelrahmanAhmed1x/core/httpclient"
	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/abdelrahmanAhmed1x/core/lifecycle"
	"github.com/abdelrahmanAhmed1x/core/logger"
	"github.com/abdelrahmanAhmed1x/core/middleware"
	"github.com/abdelrahmanAhmed1x/core/observability"
	"github.com/abdelrahmanAhmed1x/core/pagination"
	"github.com/abdelrahmanAhmed1x/core/ratelimiter"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// doc:begin models
type User struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}
type Product struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// doc:end models

func sharedSetup() {
	// doc:begin setup
	ctx := context.Background() // In a handler, use c.Request.Context().
	r := gin.New()
	r.Use(gin.Recovery())
	// doc:end setup
	_, _ = ctx, r
}

func redisSetup() {
	// doc:begin redis-client
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379", ContextTimeoutEnabled: true,
		DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
	})
	defer rdb.Close() // In main: the application owns this shared client.
	// doc:end redis-client
}

func statefulExample(ctx context.Context, rdb redis.UniversalClient, verifiedUser User) error {
	// doc:begin stateful
	auth := stateful.New[User](rdb, stateful.Config{
		Namespace: "shop:sessions", AbsoluteTTL: 24 * time.Hour,
	})
	token, err := auth.Create(ctx, "user-42", verifiedUser)
	if err != nil {
		return err
	}
	user, err := auth.Verify(ctx, token)
	if err != nil {
		return err
	}
	// doc:end stateful
	_ = user
	return nil
}
func statefulActions(ctx context.Context, auth *stateful.Manager[User], token string) error {
	// doc:begin stateful-actions
	if _, err := auth.Renew(ctx, token); err != nil {
		return err
	}
	if err := auth.Revoke(ctx, token); err != nil {
		return err
	}
	return auth.RevokeAll(ctx, "user-42")
	// doc:end stateful-actions
}
func statefulWeb(c *gin.Context, auth *stateful.Manager[User], verifiedUser User) error {
	// doc:begin stateful-web
	// After password/MFA verification in your existing login handler:
	if err := auth.Login(c, "user-42", verifiedUser); err != nil {
		return err
	}
	// In your logout handler, return auth.Logout(c).
	// doc:end stateful-web
	return nil
}

func singleExample(verifiedUser User, signingKey string) error {
	// doc:begin single
	auth := single.New[User](single.Config{
		SecretKey: signingKey, Issuer: "shop", Audience: "api", TTL: 15 * time.Minute,
	})
	token, err := auth.Issue(verifiedUser)
	if err != nil {
		return err
	}
	user, err := auth.Verify(token)
	if err != nil {
		return err
	}
	// doc:end single
	_ = user
	return nil
}
func statelessExample(verifiedUser User, signingKey, refreshKey string) error {
	// doc:begin stateless
	auth := stateless.New[User](stateless.Config{
		Issuer: "shop", AccessAudience: "api", RefreshAudience: "refresh",
		AccessKey: signingKey, RefreshKey: refreshKey,
		AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
	})
	pair, err := auth.Issue(verifiedUser)
	if err != nil {
		return err
	}
	user, err := auth.VerifyAccess(pair.Access)
	if err != nil {
		return err
	}
	// doc:end stateless
	_, err = auth.VerifyRefresh(pair.Refresh)
	if err != nil {
		return err
	}
	_ = user
	// doc:begin stateless-refresh
	nextPair, err := auth.Refresh(pair.Refresh)
	if err != nil {
		return err
	}
	// doc:end stateless-refresh
	_ = nextPair
	return nil
}
func hybridExample(c *gin.Context, rdb redis.UniversalClient, verifiedUser User, signingKey string) error {
	// doc:begin hybrid
	auth := hybrid.New[User](rdb, hybrid.Config{
		SecretKey: signingKey, Issuer: "shop", CookieSecure: true,
		AllowedOrigins: []string{"https://app.example.com"},
	})
	if err := auth.Login(c, verifiedUser); err != nil {
		return err
	}
	// In separate handlers: auth.Refresh(c) and auth.Logout(c), both return error.
	// doc:end hybrid
	return nil
}

// authRoutes deliberately accepts the common method subset only for this example,
// not as a new library interface.
func authRoutes(r *gin.Engine, auth interface {
	Middleware() gin.HandlerFunc
	Payload(*gin.Context) (User, bool)
}) {
	// doc:begin auth-route
	r.GET("/me", auth.Middleware(), func(c *gin.Context) {
		user, ok := auth.Payload(c)
		if !ok {
			c.AbortWithStatus(401)
			return
		}
		c.JSON(200, user)
	})
	// doc:end auth-route
}
func transportExample(c *gin.Context, token string) error {
	// doc:begin transport
	cookies := transport.Normalize(transport.Config{
		CookieName: "__Host-access", AllowedOrigins: []string{"https://app.example.com"},
	})
	transport.SetCookie(c.Writer, cookies, token, 15*time.Minute)
	// To delete the client's cookie: transport.SetCookie(c.Writer, cookies, "", 0).
	// doc:end transport
	raw, err := transport.Extract(c.Request, cookies)
	_, _ = raw, err
	return err
}
func memoryExample(ctx context.Context, product Product) error {
	// doc:begin memory
	local := memory.New[Product](memory.Config{Entries: 1000}, nil) // nil selects JSON.
	if err := local.Set(ctx, "product:42", product, time.Minute); err != nil {
		return err
	}
	entry, err := local.Get(ctx, "product:42")
	if err != nil {
		return err
	}
	// doc:end memory
	_ = entry.Value
	return local.Delete(ctx, "product:42")
}
func redisCacheExample(rdb redis.UniversalClient) {
	// doc:begin redis-cache
	shared := rediscache.New[Product](rdb, rediscache.Config{
		Namespace: "shop:products:v1", Timeout: time.Second,
	}, nil)
	// doc:end redis-cache
	_ = shared
}
func tieredExample(local, shared cache.Cache[Product]) {
	// doc:begin tiered
	products := tiered.New[Product](local, shared, 2*time.Second, nil)
	// doc:end tiered
	_ = products
}
func loaderExample(ctx context.Context, local cache.Cache[Product], loadProduct func(context.Context) (Product, error)) error {
	// doc:begin loader
	products := cache.NewLoader[Product](local, cache.LoaderConfig{Coalesce: true})
	product, err := products.GetOrLoad(ctx, "product:42", time.Minute, loadProduct)
	if err != nil {
		return err
	}
	// doc:end loader
	_ = product
	return nil
}
func loaderWrites(ctx context.Context, products *cache.Loader[Product], updated Product) error {
	// doc:begin loader-writes
	if err := products.Store(ctx, "product:42", updated, time.Minute); err != nil {
		return err
	}
	return products.Invalidate(ctx, "product:42")
	// doc:end loader-writes
}
func quickJWT(signingKey string) {
	// doc:begin quick-jwt
	auth := single.New[User](single.Config{
		SecretKey: signingKey, Issuer: "shop", Audience: "api", TTL: 15 * time.Minute,
	})
	// doc:end quick-jwt
	_ = auth
}

func quickCache(ctx context.Context, loadProduct func(context.Context) (Product, error)) error {
	// doc:begin quick-cache
	local := memory.New[Product](memory.Config{Entries: 1000}, nil)
	products := cache.NewLoader[Product](local, cache.LoaderConfig{Coalesce: true})
	product, err := products.GetOrLoad(ctx, "product:42", time.Minute, loadProduct)
	if err != nil {
		return err
	}
	// doc:end quick-cache
	_ = product
	return nil
}
func authorizationExample(r *gin.Engine, auth *single.Manager[User]) {
	// doc:begin authorization
	access := authorization.New(func(c *gin.Context) (authorization.Principal, error) {
		user, ok := auth.Payload(c)
		if !ok {
			return authorization.Principal{}, authorization.ErrUnauthenticated
		}
		return authorization.Principal{Roles: []string{user.Role}}, nil
	})
	r.GET("/admin", auth.Middleware(), access.RequireRole("admin"), func(c *gin.Context) {
		c.Status(204)
	})
	// doc:end authorization
}
func httpxExample(c *gin.Context) {
	// doc:begin httpx
	request, ok := httpx.BindJSON[struct {
		Name string `json:"name" binding:"required"`
	}](c)
	if !ok {
		return
	} // A 400 response has already been sent.
	httpx.Created(c, Product{ID: 42, Name: request.Name})
	// doc:end httpx
}
func loggerExample(r *gin.Engine, ctx context.Context) {
	// doc:begin logger
	log := logger.New(logger.Config{Format: "json", MaskSensitives: true})
	r.Use(logger.Middleware(log))
	log.Info(ctx, "service started", "component", "api")
	// doc:end logger
}
func middlewareExample(r *gin.Engine) error {
	// doc:begin middleware
	if err := r.SetTrustedProxies(nil); err != nil {
		return err
	}
	r.Use(middleware.RequestID(), middleware.SecurityHeaders())
	r.Use(middleware.MaxBodySize(1<<20), middleware.Timeout(3*time.Second))
	// doc:end middleware
	return nil
}
func corsExample(r *gin.Engine) {
	// doc:begin cors
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"}, AllowCredentials: true,
	}))
	// doc:end cors
}
func paginationExample(c *gin.Context) {
	// doc:begin pagination
	q, ok := httpx.BindQuery[pagination.Query](c)
	if !ok {
		return
	}
	// Pass q.Offset() and q.Limit to your database query.
	items, total := []Product{{ID: 42, Name: "Keyboard"}}, 1
	httpx.OK(c, pagination.NewResult(items, total, q))
	// doc:end pagination
}
func limiterExample(r *gin.Engine, rdb redis.UniversalClient) {
	// doc:begin ratelimiter
	limiter := ratelimiter.New(rdb)
	r.Use(limiter.Middleware(ratelimiter.Limit{
		Rate: 100, Burst: 20, Period: time.Minute, FailureMode: ratelimiter.FailClosed,
	}, ratelimiter.ByIP))
	// doc:end ratelimiter
}
func healthExample(r *gin.Engine, rdb redis.UniversalClient) {
	// doc:begin health
	checks := health.New(time.Second)
	checks.Register("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() })
	r.GET("/live", checks.LiveHandler())
	r.GET("/ready", checks.ReadyHandler())
	// doc:end health
}
func clientExample(ctx context.Context) error {
	// doc:begin httpclient
	client := httpclient.New(nil, httpclient.Config{Timeout: 3 * time.Second, Retries: 2})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// doc:end httpclient
	return nil
}
func observabilityExample(r *gin.Engine, provider metric.MeterProvider, tracer trace.Tracer) error {
	// doc:begin observability
	metrics, err := observability.New(provider.Meter("shop"))
	if err != nil {
		return err
	}
	r.Use(metrics.Middleware(tracer, propagation.TraceContext{}))
	local := memory.New[Product](memory.Config{Entries: 1000, Observer: metrics}, nil)
	// doc:end observability
	_ = local
	return nil
}
func lifecycleExample(rdb redis.UniversalClient, server *http.Server) error {
	// doc:begin lifecycle
	shutdown := lifecycle.New(10 * time.Second)
	if err := shutdown.Register("redis", func(context.Context) error { return rdb.Close() }); err != nil {
		return err
	}
	if err := shutdown.RegisterHTTP("http", server); err != nil {
		return err
	}
	// Call after receiving a shutdown signal, using a fresh context:
	return shutdown.Shutdown(context.Background())
	// doc:end lifecycle
}

// Example demonstrates the memory + loader quick start without Redis or a server.
func Example() {
	local := memory.New[Product](memory.Config{Entries: 1000}, nil)
	products := cache.NewLoader[Product](local, cache.LoaderConfig{Coalesce: true})
	product, err := products.GetOrLoad(context.Background(), "product:42", time.Minute,
		func(context.Context) (Product, error) { return Product{42, "Keyboard"}, nil })
	if err != nil {
		panic(err)
	}
	fmt.Println(product.Name)
	// Output: Keyboard
}

// TestSnippetSync makes changes to the documented APIs compile with go test and
// checks that each marked README block matches this source instead of drifting.
func TestSnippetSync(t *testing.T) {
	checkSnippets(t)
}

func TestNoopMetricsExample(t *testing.T) {
	if err := observabilityExample(gin.New(), noop.NewMeterProvider(), nil); err != nil {
		t.Fatal(err)
	}
}
