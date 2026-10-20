package main

import (
	"context"
	"fmt"
	"github.com/abdelrahmanAhmed1x/core/cache"
	"github.com/abdelrahmanAhmed1x/core/cache/memory"
	cr "github.com/abdelrahmanAhmed1x/core/cache/redis"
	"github.com/abdelrahmanAhmed1x/core/cache/tiered"
	"github.com/redis/go-redis/v9"
	"time"
)

func main() {
	local := memory.New[string](memory.Config{Entries: 1000, MaxBytes: 64 << 10}, nil)
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", ContextTimeoutEnabled: true})
	defer rdb.Close()
	distributed := cr.New[string](rdb, cr.Config{Namespace: "example:v1", Timeout: time.Second}, nil)
	// Each backend works alone. The tiered constructor merely composes them.
	_ = tiered.New[string](local, distributed, 2*time.Second, nil)
	loader := cache.NewLoader[string](local, cache.LoaderConfig{Coalesce: true, Jitter: 0.1})
	value, err := loader.GetOrLoad(context.Background(), "greeting", time.Minute, func(context.Context) (string, error) { return "hello", nil })
	if err != nil {
		panic(err)
	}
	fmt.Println(value)
}
