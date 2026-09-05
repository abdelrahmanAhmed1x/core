package jwt

import (
	"github.com/redis/go-redis/v9"
)

type manager[T any] struct {
	redis redis.UniversalClient
	cfg   Config
}
