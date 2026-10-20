// Package testredis is restricted to opt-in integration tests on loopback stores.
package testredis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"net"
	"os"
	"testing"
	"time"
)

func Client(t testing.TB) *redis.Client {
	t.Helper()
	if os.Getenv("CORE_INTEGRATION") != "1" {
		t.Skip("set CORE_INTEGRATION=1 and CORE_REDIS_ADDR for disposable Redis/Dragonfly")
	}
	addr := os.Getenv("CORE_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:16379"
	}
	h, _, e := net.SplitHostPort(addr)
	if e != nil || !net.ParseIP(h).IsLoopback() {
		t.Fatal("integration address must be a loopback IP")
	}
	c := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1, ReadTimeout: time.Second, WriteTimeout: time.Second, ContextTimeoutEnabled: true})
	if e = c.Ping(context.Background()).Err(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
