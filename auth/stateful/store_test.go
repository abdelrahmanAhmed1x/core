package stateful_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/stateful"
)

// This implementation lives outside package stateful to verify that the public
// contract is sufficient without access to internal Redis or token helpers.
type customStore struct {
	create    func(context.Context, stateful.CreateRequest) error
	verify    func(context.Context, string, string) ([]byte, error)
	renew     func(context.Context, string, string, time.Duration) ([]byte, error)
	revoke    func(context.Context, string, string) error
	revokeAll func(context.Context, string, string, time.Duration) error
}

func (s customStore) Create(ctx context.Context, r stateful.CreateRequest) error {
	return s.create(ctx, r)
}
func (s customStore) Verify(ctx context.Context, ns, hash string) ([]byte, error) {
	return s.verify(ctx, ns, hash)
}
func (s customStore) Renew(ctx context.Context, ns, hash string, ttl time.Duration) ([]byte, error) {
	return s.renew(ctx, ns, hash, ttl)
}
func (s customStore) Revoke(ctx context.Context, ns, hash string) error {
	return s.revoke(ctx, ns, hash)
}
func (s customStore) RevokeAll(ctx context.Context, ns, hash string, ttl time.Duration) error {
	return s.revokeAll(ctx, ns, hash, ttl)
}
func digest(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func TestCustomStoreBoundary(t *testing.T) {
	cfg := stateful.Config{Namespace: "custom", AbsoluteTTL: time.Hour, IdleTTL: time.Minute, OperationTimeout: time.Second}
	type payload struct{ ID int64 }
	want := payload{ID: 9007199254740993}
	var request stateful.CreateRequest
	checkContext := func(ctx context.Context) {
		t.Helper()
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > cfg.OperationTimeout {
			t.Fatal("store did not receive operation deadline")
		}
	}
	s := customStore{create: func(ctx context.Context, r stateful.CreateRequest) error {
		checkContext(ctx)
		request = r
		return nil
	}}
	m := stateful.NewWithStore[payload](&s, cfg)
	ctx := context.Background()
	raw, err := m.Create(ctx, "alice", want)
	if err != nil {
		t.Fatal(err)
	}
	if request.Namespace != cfg.Namespace || request.TokenHash != digest(raw) || request.UserHash != digest("alice") || request.AbsoluteTTL != cfg.AbsoluteTTL || request.IdleTTL != cfg.IdleTTL {
		t.Fatalf("incorrect storage request: %+v", request)
	}
	if string(request.Payload) != `{"ID":9007199254740993}` {
		t.Fatalf("JSON precision lost: %s", request.Payload)
	}
	checkKey := func(ctx context.Context, ns, hash string) {
		t.Helper()
		checkContext(ctx)
		if ns != cfg.Namespace || hash != digest(raw) {
			t.Fatal("incorrect namespace or credential hash")
		}
	}
	s.verify = func(ctx context.Context, ns, hash string) ([]byte, error) {
		checkKey(ctx, ns, hash)
		return request.Payload, nil
	}
	s.renew = func(ctx context.Context, ns, hash string, ttl time.Duration) ([]byte, error) {
		checkKey(ctx, ns, hash)
		if ttl != cfg.IdleTTL {
			t.Fatal("incorrect idle TTL")
		}
		return request.Payload, nil
	}
	s.revoke = func(ctx context.Context, ns, hash string) error {
		checkKey(ctx, ns, hash)
		return nil
	}
	s.revokeAll = func(ctx context.Context, ns, hash string, ttl time.Duration) error {
		checkContext(ctx)
		if ns != cfg.Namespace || hash != digest("alice") || ttl != cfg.AbsoluteTTL {
			t.Fatal("incorrect user revocation request")
		}
		return nil
	}
	for _, read := range []func(context.Context, string) (payload, error){m.Verify, m.Renew} {
		if got, err := read(ctx, raw); err != nil || got != want {
			t.Fatal(got, err)
		}
	}
	if err := m.Revoke(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if err := m.RevokeAll(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
}

func TestCustomStoreCollisions(t *testing.T) {
	for _, collisions := range []int{2, 3} {
		t.Run(strconv.Itoa(collisions), func(t *testing.T) {
			seen := map[string]bool{}
			s := customStore{create: func(_ context.Context, r stateful.CreateRequest) error {
				if seen[r.TokenHash] {
					t.Fatal("collision retry reused credential")
				}
				seen[r.TokenHash] = true
				if len(seen) <= collisions {
					return stateful.ErrCollision
				}
				return nil
			}}
			m := stateful.NewWithStore[int](s, stateful.Config{Namespace: "test", AbsoluteTTL: time.Hour})
			raw, err := m.Create(context.Background(), "alice", 1)
			if len(seen) != 3 {
				t.Fatal("unexpected retry count", len(seen))
			}
			if collisions == 3 {
				if raw != "" || !errors.Is(err, stateful.ErrCollision) {
					t.Fatal(raw, err)
				}
			} else if raw == "" || err != nil {
				t.Fatal(raw, err)
			}
		})
	}
}

func TestCustomStoreErrorsAndValidation(t *testing.T) {
	outage := errors.New("store unavailable")
	s := customStore{}
	m := stateful.NewWithStore[int](&s, stateful.Config{Namespace: "test", AbsoluteTTL: time.Hour})
	ctx := context.Background()
	// Nil callbacks ensure invalid input is rejected before storage is called.
	if _, err := m.Create(ctx, "", 1); err == nil {
		t.Fatal("empty user accepted")
	}
	for _, read := range []func(context.Context, string) (int, error){m.Verify, m.Renew} {
		if _, err := read(ctx, "invalid"); !errors.Is(err, stateful.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if !errors.Is(m.Revoke(ctx, "invalid"), stateful.ErrInvalid) || !errors.Is(m.RevokeAll(ctx, ""), stateful.ErrInvalid) {
		t.Fatal("invalid revocation input accepted")
	}
	s.create = func(ctx context.Context, _ stateful.CreateRequest) error { return ctx.Err() }
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.Create(cancelled, "alice", 1); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	s.create = func(context.Context, stateful.CreateRequest) error { return outage }
	if _, err := m.Create(ctx, "alice", 1); !errors.Is(err, outage) {
		t.Fatal("creation error lost", err)
	}
	const raw = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	for _, storeErr := range []error{outage, stateful.ErrInvalid} {
		s.verify = func(context.Context, string, string) ([]byte, error) { return nil, storeErr }
		s.renew = func(context.Context, string, string, time.Duration) ([]byte, error) { return nil, storeErr }
		s.revoke = func(context.Context, string, string) error { return storeErr }
		s.revokeAll = func(context.Context, string, string, time.Duration) error { return storeErr }
		for _, read := range []func(context.Context, string) (int, error){m.Verify, m.Renew} {
			if _, err := read(ctx, raw); !errors.Is(err, storeErr) {
				t.Fatal("read error lost", err)
			}
		}
		if !errors.Is(m.Revoke(ctx, raw), storeErr) || !errors.Is(m.RevokeAll(ctx, "alice"), storeErr) {
			t.Fatal("revocation error lost")
		}
	}
	s.verify = func(context.Context, string, string) ([]byte, error) { return []byte(`{"broken"`), nil }
	if _, err := m.Verify(ctx, raw); !errors.Is(err, stateful.ErrInvalid) {
		t.Fatal("corrupt payload accepted", err)
	}
	unsupported := stateful.NewWithStore[chan int](customStore{}, stateful.Config{Namespace: "test", AbsoluteTTL: time.Hour})
	if _, err := unsupported.Create(ctx, "alice", make(chan int)); err == nil {
		t.Fatal("unserializable payload accepted")
	}
}
