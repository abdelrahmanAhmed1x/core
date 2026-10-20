package token

import (
	jwt "github.com/golang-jwt/jwt/v5"
	"strings"
	"testing"
	"time"
)

func config() Config {
	return Normalize(Config{SecretKey: strings.Repeat("k", 32), Issuer: "issuer", Audience: "aud", TTL: time.Minute})
}
func TestVerification(t *testing.T) {
	c := config()
	raw, e := Issue(c, 42, "access")
	if e != nil {
		t.Fatal(e)
	}
	v, e := Verify[int](c, raw, "access")
	if e != nil || v != 42 {
		t.Fatal(v, e)
	}
	if _, e = Verify[int](c, raw, "refresh"); e == nil {
		t.Fatal("type confusion")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Claims[int])
		alg    jwt.SigningMethod
		key    any
	}{
		{"expired", func(c *Claims[int]) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }, jwt.SigningMethodHS256, []byte(c.SecretKey)},
		{"issuer", func(c *Claims[int]) { c.Issuer = "other" }, jwt.SigningMethodHS256, []byte(c.SecretKey)},
		{"audience", func(c *Claims[int]) { c.Audience = jwt.ClaimStrings{"other"} }, jwt.SigningMethodHS256, []byte(c.SecretKey)},
		{"missing expiration", func(c *Claims[int]) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, []byte(c.SecretKey)},
		{"missing issued", func(c *Claims[int]) { c.IssuedAt = nil }, jwt.SigningMethodHS256, []byte(c.SecretKey)},
		{"forged", func(*Claims[int]) {}, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32))},
		{"algorithm", func(*Claims[int]) {}, jwt.SigningMethodHS384, []byte(c.SecretKey)},
		{"none", func(*Claims[int]) {}, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := Claims[int]{Payload: 42, Type: "access", RegisteredClaims: jwt.RegisteredClaims{Issuer: c.Issuer, Audience: jwt.ClaimStrings{c.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now()), ID: "id"}}
			tc.mutate(&cl)
			s, e := jwt.NewWithClaims(tc.alg, cl).SignedString(tc.key)
			if e != nil {
				t.Fatal(e)
			}
			if _, e := Verify[int](c, s, "access"); e == nil {
				t.Fatal("accepted invalid claims")
			}
		})
	}
}
func TestOpaque(t *testing.T) {
	s, e := Random()
	if e != nil || !ValidOpaque(s) || len(Hash(s)) != 64 {
		t.Fatal(s, e)
	}
	for _, s := range []string{"", "x", strings.Repeat("!", 43), strings.Repeat("A", 100000)} {
		if ValidOpaque(s) {
			t.Fatal("accepted malformed opaque")
		}
	}
}
func FuzzVerify(f *testing.F) {
	f.Add("a.b.c")
	c := config()
	f.Fuzz(func(t *testing.T, s string) { Verify[int](c, s, "access") })
}
func BenchmarkVerify(b *testing.B) {
	c := config()
	s, _ := Issue(c, 42, "access")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, e := Verify[int](c, s, "access"); e != nil {
			b.Fatal(e)
		}
	}
}
