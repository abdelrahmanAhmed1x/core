// Command auth demonstrates independent authentication managers. No credential is
// logged. Set CORE_EXAMPLE_REDIS=1 to exercise an explicitly disposable local store.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	hybrid "github.com/abdelrahmanAhmed1x/core/auth/jwt"
	"github.com/abdelrahmanAhmed1x/core/auth/single"
	"github.com/abdelrahmanAhmed1x/core/auth/stateful"
	"github.com/abdelrahmanAhmed1x/core/auth/stateless"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

type User struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}

func key() string {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func main() {
	// Load stable high-entropy keys from your secret manager in real deployments.
	accessKey, refreshKey := key(), key()
	user := User{42, "reader"}
	one := single.New[User](single.Config{SecretKey: accessKey, Issuer: "example", Audience: "api", TTL: 5 * time.Minute})
	credential, e := one.Issue(user)
	if e != nil {
		panic(e)
	}
	verified, e := one.Verify(credential)
	if e != nil {
		panic(e)
	}
	fmt.Println("single verified role:", verified.Role)
	pair := stateless.New[User](stateless.Config{Issuer: "example", AccessAudience: "api", RefreshAudience: "refresh", AccessKey: accessKey, RefreshKey: refreshKey, AccessTTL: 5 * time.Minute, RefreshTTL: time.Hour})
	tokens, e := pair.Issue(user)
	if e != nil {
		panic(e)
	}
	if _, e = pair.Refresh(tokens.Refresh); e != nil {
		panic(e)
	}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", ContextTimeoutEnabled: true})
	defer rdb.Close()
	sessions := stateful.New[User](rdb, stateful.Config{Namespace: "example", AbsoluteTTL: 24 * time.Hour, IdleTTL: 30 * time.Minute})
	_ = hybrid.New[User](rdb, hybrid.Config{SecretKey: accessKey, Issuer: "example", CookieSecure: true, AllowedOrigins: []string{"https://app.example"}})
	if os.Getenv("CORE_EXAMPLE_REDIS") == "1" {
		ctx := context.Background()
		s, e := sessions.Create(ctx, "42", user)
		if e != nil {
			panic(e)
		}
		if _, e = sessions.Verify(ctx, s); e != nil {
			panic(e)
		}
		if e = sessions.RevokeAll(ctx, "42"); e != nil {
			panic(e)
		}
	}
}
