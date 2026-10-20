package ratelimiter

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"math"
)

// redis_rate expects Redis's integer Lua replies. Dragonfly RESP3 can preserve
// fractional Lua numbers. Normalize the two count fields using Redis's truncation
// semantics, while validating the reply before redis_rate's type assertions.
type compatibleClient struct{ redis.UniversalClient }

func normalize(cmd *redis.Cmd) *redis.Cmd {
	if cmd.Err() != nil {
		return cmd
	}
	v, e := cmd.Slice()
	if e != nil || len(v) != 4 {
		cmd.SetErr(fmt.Errorf("ratelimiter: invalid script reply"))
		return cmd
	}
	for i := range 2 {
		switch n := v[i].(type) {
		case int64:
		case float64:
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n >= float64(math.MaxInt64) {
				cmd.SetErr(fmt.Errorf("ratelimiter: invalid count"))
				return cmd
			}
			v[i] = int64(n)
		default:
			cmd.SetErr(fmt.Errorf("ratelimiter: invalid count type"))
			return cmd
		}
	}
	for i := 2; i < 4; i++ {
		if _, ok := v[i].(string); !ok {
			cmd.SetErr(fmt.Errorf("ratelimiter: invalid duration type"))
			return cmd
		}
	}
	cmd.SetVal(v)
	return cmd
}
func (c compatibleClient) Eval(ctx context.Context, s string, k []string, a ...any) *redis.Cmd {
	return normalize(c.UniversalClient.Eval(ctx, s, k, a...))
}
func (c compatibleClient) EvalSha(ctx context.Context, s string, k []string, a ...any) *redis.Cmd {
	return normalize(c.UniversalClient.EvalSha(ctx, s, k, a...))
}
func (c compatibleClient) EvalRO(ctx context.Context, s string, k []string, a ...any) *redis.Cmd {
	return normalize(c.UniversalClient.EvalRO(ctx, s, k, a...))
}
func (c compatibleClient) EvalShaRO(ctx context.Context, s string, k []string, a ...any) *redis.Cmd {
	return normalize(c.UniversalClient.EvalShaRO(ctx, s, k, a...))
}
