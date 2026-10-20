// Package httpclient adds bounded, cancellation-aware retries to net/http.
package httpclient

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Config struct {
	Timeout     time.Duration
	Retries     int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// RetryNonIdempotent is an explicit application opt-in; bodies must be replayable.
	RetryNonIdempotent bool
	RetryStatus        func(int) bool
	RetryError         func(error) bool
	RequestID          func(context.Context) string
	// OnAttempt receives status, duration and error, never headers or credentials.
	OnAttempt func(context.Context, int, time.Duration, error)
}
type Client struct {
	client *http.Client
	cfg    Config
}

// New copies a caller-owned client. A nil client uses a cloned standard
// Transport, whose idle connections may be closed via CloseIdleConnections.
func New(client *http.Client, c Config) *Client {
	if c.Timeout < 0 || c.Retries < 0 || c.Retries > 10 || c.BaseBackoff < 0 || c.MaxBackoff < 0 || c.MaxBackoff > time.Minute {
		panic("httpclient: invalid configuration")
	}
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	if c.BaseBackoff == 0 {
		c.BaseBackoff = 100 * time.Millisecond
	}
	if c.MaxBackoff == 0 {
		c.MaxBackoff = 5 * time.Second
	}
	if c.MaxBackoff < c.BaseBackoff {
		panic("httpclient: invalid backoff")
	}
	if c.RetryStatus == nil {
		c.RetryStatus = func(s int) bool { return s == 429 || s == 502 || s == 503 || s == 504 }
	}
	if c.RetryError == nil {
		c.RetryError = func(e error) bool { var n net.Error; return errors.As(e, &n) && n.Timeout() }
	}
	var owned http.Client
	if client != nil {
		owned = *client
	} else {
		owned.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	return &Client{&owned, c}
}
func (c *Client) CloseIdleConnections() { c.client.CloseIdleConnections() }
func idempotent(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS", "TRACE", "PUT", "DELETE":
		return true
	}
	return false
}
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("httpclient: nil request")
	}
	ctx, cancel := context.WithTimeout(req.Context(), c.cfg.Timeout)
	// Successful response bodies retain the deadline until closed by the caller.
	attempts := c.cfg.Retries
	if (!idempotent(req.Method) && !c.cfg.RetryNonIdempotent) || (req.Body != nil && req.Body != http.NoBody && req.GetBody == nil) {
		attempts = 0
	}
	backoff := c.cfg.BaseBackoff
	for attempt := 0; ; attempt++ {
		r := req.Clone(ctx)
		if r.Header == nil {
			r.Header = make(http.Header)
		}
		if attempt > 0 && req.GetBody != nil {
			body, e := req.GetBody()
			if e != nil {
				cancel()
				return nil, e
			}
			r.Body = body
		}
		if c.cfg.RequestID != nil {
			if id := c.cfg.RequestID(ctx); id != "" {
				r.Header.Set("X-Request-ID", id)
			}
		}
		start := time.Now()
		resp, e := c.client.Do(r)
		if c.cfg.OnAttempt != nil {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			c.cfg.OnAttempt(ctx, status, time.Since(start), e)
		}
		retry := e != nil && c.cfg.RetryError(e)
		if e == nil {
			retry = c.cfg.RetryStatus(resp.StatusCode)
		}
		if !retry || attempt == attempts || ctx.Err() != nil {
			if e != nil {
				cancel()
				return resp, e
			}
			resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}
		delay := time.Duration(rand.Int64N(int64(backoff) + 1))
		if resp != nil {
			if d := retryAfter(resp.Header.Get("Retry-After")); d > delay {
				if d > c.cfg.MaxBackoff {
					resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
					return resp, nil
				}
				delay = d
			}
			resp.Body.Close()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			cancel()
			return nil, ctx.Err()
		case <-timer.C:
		}
		if backoff < c.cfg.MaxBackoff/2 {
			backoff *= 2
		} else {
			backoff = c.cfg.MaxBackoff
		}
	}
}
func retryAfter(s string) time.Duration {
	if n, e := strconv.ParseInt(s, 10, 64); e == nil {
		if n >= 86400 {
			return 24 * time.Hour
		}
		if n > 0 {
			return time.Duration(n) * time.Second
		}
		return 0
	}
	if t, e := http.ParseTime(s); e == nil {
		return max(0, time.Until(t))
	}
	return 0
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }
