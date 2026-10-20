package testredis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"net"
)

// Hook injects command failures without modifying the datastore protocol.
type Hook struct {
	Process func(context.Context, redis.Cmder, redis.ProcessHook) error
}

func (h Hook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (h Hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, c redis.Cmder) error { return h.Process(ctx, c, next) }
}
func (h Hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
