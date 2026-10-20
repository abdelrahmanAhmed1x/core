package jwt

import (
	"errors"
	"github.com/abdelrahmanAhmed1x/core/auth/transport"
	"github.com/gin-gonic/gin"
	gjwt "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"net/http"
	"strings"
	"time"
)

// All new family keys share a hash slot. Legacy keys retain their original names.
func (m *manager[T]) familyKeys(raw string) []string {
	p := strings.Split(raw, ".")
	if len(p) != 2 {
		return []string{m.refreshKey(raw), m.refreshKey(raw) + ":family"}
	}
	base := "auth:" + m.cfg.Issuer + ":family:{" + hashRefreshToken(p[0]) + "}:"
	return []string{base + hashRefreshToken(raw), base + "state"}
}
func (m *manager[T]) familyTTL() time.Duration {
	if m.cfg.FamilyTTL > 0 {
		return m.cfg.FamilyTTL
	}
	return m.cfg.RefreshTTL
}
func (m *manager[T]) replayGrace() time.Duration {
	if m.cfg.ReplayGrace > 0 {
		return m.cfg.ReplayGrace
	}
	return 5 * time.Second
}

var familyCreate = redis.NewScript(`
if redis.call('EXISTS',KEYS[1])==1 or redis.call('EXISTS',KEYS[2])==1 then return 0 end
redis.call('SET',KEYS[1],ARGV[1],'PX',math.min(tonumber(ARGV[2]),tonumber(ARGV[4])));redis.call('SET',KEYS[2],ARGV[3],'PX',ARGV[2]);return 1`)
var familyRotate = redis.NewScript(`
if redis.call('GET',KEYS[1])~=ARGV[1] or redis.call('EXISTS',KEYS[2])==1 then return 0 end
local active=redis.call('GET',KEYS[3]);if not active and ARGV[5]=='1' then return 0 end;if active and active~=ARGV[2] then return 0 end
local ttl=redis.call('PTTL',KEYS[3]);if not active then ttl=redis.call('PTTL',KEYS[1]) end
if ttl<=0 then return 0 end
local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000)
redis.call('SET',KEYS[2],ARGV[1],'PX',math.min(ttl,tonumber(ARGV[4])))
redis.call('SET',KEYS[4],ARGV[3],'PX',ttl)
if active then redis.call('SET',KEYS[1],'USED:'..string.format('%.0f',now),'PX',ttl) else redis.call('DEL',KEYS[1]) end
return 1`)
var familyReplay = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw or string.sub(raw,1,5)~='USED:' then return 0 end
local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000)
if now-tonumber(string.sub(raw,6))>tonumber(ARGV[1]) then redis.call('SET',KEYS[2],'REVOKED','XX','KEEPTTL');return 1 end
return 0`)

func containsAudience(a gjwt.ClaimStrings, v string) bool {
	for _, s := range a {
		if s == v {
			return true
		}
	}
	return false
}
func (m *manager[T]) signAccess(p T) (string, error) {
	now := time.Now()
	c := claims[T]{Payload: p, Type: "access", RegisteredClaims: gjwt.RegisteredClaims{Issuer: m.cfg.Issuer, IssuedAt: gjwt.NewNumericDate(now), ExpiresAt: gjwt.NewNumericDate(now.Add(m.cfg.AccessTTL))}}
	if m.cfg.Audience != "" {
		c.Audience = gjwt.ClaimStrings{m.cfg.Audience}
	}
	return gjwt.NewWithClaims(gjwt.SigningMethodHS256, c).SignedString([]byte(m.cfg.SecretKey))
}
func (m *manager[T]) deliver(c *gin.Context, a, r string) {
	setCookie(c, accessCookieName, a, m.cfg.AccessTTL, m.cfg.CookiePath, m.cfg.CookieDomain, m.cfg.CookieSecure, m.cfg.CookieSameSite)
	setCookie(c, refreshCookieName, r, m.familyTTL(), m.cfg.RefreshTokenPath, m.cfg.CookieDomain, m.cfg.CookieSecure, m.cfg.CookieSameSite)
}
func (m *manager[T]) transportConfig(name string) transport.Config {
	origins := m.cfg.AllowedOrigins
	// Legacy cookie behavior remains compatible. New applications should configure
	// explicit origins; bearer-only requests do not require CSRF checks.
	if len(origins) == 0 {
		origins = []string{""}
	}
	return transport.Config{CookieName: name, AllowedOrigins: origins, SameSite: http.SameSiteLaxMode}
}

func (m *manager[T]) extract(c *gin.Context, name string) (string, error) {
	raw, e := transport.Extract(c.Request, m.transportConfig(name))
	if errors.Is(e, transport.ErrCSRF) && len(m.cfg.AllowedOrigins) == 0 {
		r := c.Request.Clone(c.Request.Context())
		r.Method = "GET"
		return transport.Extract(r, m.transportConfig(name))
	}
	return raw, e
}
