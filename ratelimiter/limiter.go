package ratelimiter

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/abdelrahmanAhmed1x/core/httpx"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type FailureMode uint8

const (
	FailOpen FailureMode = iota
	FailClosed
)

var (
	errInvalidRate        = errors.New("ratelimiter: rate must be greater than 0")
	errInvalidBurst       = errors.New("ratelimiter: burst must be greater than 0")
	errInvalidPeriod      = errors.New("ratelimiter: period must be greater than 0")
	errInvalidFailureMode = errors.New("ratelimiter: invalid failure mode")
	errNilKeyFunc         = errors.New("ratelimiter: key function cannot be nil")
)

type limiter struct {
	limiter *redis_rate.Limiter
}

func New(client redis.UniversalClient) *limiter {
	return &limiter{
		limiter: redis_rate.NewLimiter(client),
	}
}

type KeyFunc func(*gin.Context) string

type Limit struct {
	Rate        int
	Burst       int
	Period      time.Duration
	FailureMode FailureMode
}

func (l Limit) validate() error {
	if l.Rate <= 0 {
		return errInvalidRate
	}

	if l.Burst <= 0 {
		return errInvalidBurst
	}

	if l.Period <= 0 {
		return errInvalidPeriod
	}

	switch l.FailureMode {
	case FailOpen, FailClosed:
		// valid
	default:
		return errInvalidFailureMode
	}

	return nil
}

func validateMiddleware(limit Limit, keyFn KeyFunc) error {
	if err := limit.validate(); err != nil {
		return fmt.Errorf("invalid limit: %w", err)
	}

	if keyFn == nil {
		return errNilKeyFunc
	}

	return nil
}

func (l *Limit) redisLimit() redis_rate.Limit {
	return redis_rate.Limit{
		Rate:   l.Rate,
		Burst:  l.Burst,
		Period: l.Period,
	}
}

func (l *limiter) Middleware(
	limit Limit,
	keyfn KeyFunc,
) gin.HandlerFunc {
	if err := validateMiddleware(limit, keyfn); err != nil {
		panic(err)
	}

	return func(c *gin.Context) {
		key := keyfn(c)

		result, err := l.limiter.Allow(
			c.Request.Context(),
			key,
			limit.redisLimit(),
		)
		if err != nil {
			if limit.FailureMode == FailOpen {
				c.Next()
				return
			}
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if result.Allowed == 0 {
			httpx.AbortTooManyRequests(c)
			return
		}
		c.Next()
	}
}

func ByIP(c *gin.Context) string {
	return c.ClientIP()
}
