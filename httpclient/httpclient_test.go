package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type tracked struct {
	io.Reader
	closed *atomic.Int32
}

func (t tracked) Close() error { t.closed.Add(1); return nil }
func TestRetriesBodies(t *testing.T) {
	var calls, closed atomic.Int32
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if b, e := io.ReadAll(r.Body); e != nil || string(b) != "body" {
			t.Error(string(b), e)
		}
		r.Body.Close()
		status := 503
		if calls.Load() == 3 {
			status = 200
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: tracked{strings.NewReader("ok"), &closed}}, nil
	})
	c := New(&http.Client{Transport: transport}, Config{Retries: 2, BaseBackoff: time.Millisecond})
	r, _ := http.NewRequest("PUT", "http://example", strings.NewReader("body"))
	resp, e := c.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if calls.Load() != 3 || closed.Load() != 3 {
		t.Fatal(calls.Load(), closed.Load())
	}
}
func TestPostNotRetried(t *testing.T) {
	var n atomic.Int32
	c := New(&http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		n.Add(1)
		return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
	})}, Config{Retries: 3})
	r, _ := http.NewRequest("POST", "http://example", nil)
	resp, e := c.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if n.Load() != 1 {
		t.Fatal(n.Load())
	}
}
func TestCancellationAndDeadline(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	c := New(nil, Config{Retries: 2, Timeout: 20 * time.Millisecond})
	r, _ := http.NewRequest("GET", s.URL, nil)
	_, e := c.Do(r)
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}
func TestUnreplayable(t *testing.T) {
	var n atomic.Int32
	c := New(&http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		n.Add(1)
		r.Body.Close()
		return &http.Response{StatusCode: 503, Header: make(http.Header), Body: http.NoBody}, nil
	})}, Config{Retries: 2})
	r, _ := http.NewRequest("PUT", "http://example", io.NopCloser(strings.NewReader("body")))
	resp, e := c.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if n.Load() != 1 {
		t.Fatal(n.Load())
	}
}
func TestRetryAfter(t *testing.T) {
	if d := retryAfter("2"); d != 2*time.Second {
		t.Fatal(d)
	}
	if d := retryAfter(time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)); d < 59*time.Second {
		t.Fatal(d)
	}
	if d := retryAfter("nonsense"); d != 0 {
		t.Fatal(d)
	}
}

func TestRetryAfterBudget(t *testing.T) {
	var n atomic.Int32
	c := New(&http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		n.Add(1)
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"3600"}}, Body: http.NoBody}, nil
	})}, Config{Retries: 2, MaxBackoff: time.Second})
	r, _ := http.NewRequest("GET", "http://example", nil)
	resp, e := c.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if n.Load() != 1 {
		t.Fatal("retried earlier than Retry-After")
	}
}

func TestRequestIDWithNilHeaders(t *testing.T) {
	seen := false
	c := New(&http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		seen = r.Header.Get("X-Request-ID") == "request-1"
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: http.NoBody}, nil
	})}, Config{RequestID: func(context.Context) string { return "request-1" }})
	r, _ := http.NewRequest("GET", "http://example", nil)
	r.Header = nil
	resp, e := c.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if !seen || r.Header != nil {
		t.Fatal("header propagation or caller mutation")
	}
	c.CloseIdleConnections()
}
