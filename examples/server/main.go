// Command server is a runnable bearer-auth API. JWT issuance belongs in an
// application login flow after password/MFA verification; no demo login bypass
// is mounted here. Set CORE_SIGNING_KEY to a stable random key of >=32 bytes.
package main

import (
	"context"
	"errors"
	"github.com/abdelrahmanAhmed1x/core/auth/single"
	"github.com/abdelrahmanAhmed1x/core/authorization"
	"github.com/abdelrahmanAhmed1x/core/health"
	"github.com/abdelrahmanAhmed1x/core/lifecycle"
	"github.com/abdelrahmanAhmed1x/core/middleware"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type User struct {
	Role string `json:"role"`
}

func main() {
	auth := single.New[User](single.Config{SecretKey: os.Getenv("CORE_SIGNING_KEY"), Issuer: "example", Audience: "api", TTL: 5 * time.Minute})
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.SecurityHeaders())
	if e := r.SetTrustedProxies(nil); e != nil {
		log.Fatal(e)
	}
	h := health.New(time.Second)
	r.GET("/live", h.LiveHandler())
	r.GET("/ready", h.ReadyHandler())
	policy := authorization.New(func(c *gin.Context) (authorization.Principal, error) {
		p, ok := auth.Payload(c)
		if !ok {
			return authorization.Principal{}, authorization.ErrUnauthenticated
		}
		return authorization.Principal{Roles: []string{p.Role}}, nil
	})
	r.GET("/admin", auth.Middleware(), policy.RequireRole("admin"), func(c *gin.Context) { c.Status(204) })
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
	stop := lifecycle.New(10 * time.Second)
	if e := stop.RegisterHTTP("http", server); e != nil {
		log.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case e := <-done:
		if !errors.Is(e, http.ErrServerClosed) {
			log.Print("server stopped unexpectedly")
		}
	}
	if e := stop.Shutdown(context.Background()); e != nil {
		log.Print("shutdown failed")
	}
}
