package stateful

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/abdelrahmanAhmed1x/core/auth/internal/token"
	"github.com/redis/go-redis/v9"
)

// RedisStore implements SessionStore using atomic Redis scripts. It also supports
// DragonflyDB. The caller owns the client and its lifecycle.
type RedisStore struct{ client redis.UniversalClient }

var _ SessionStore = (*RedisStore)(nil)

// NewRedisStore wraps a client without changing its configuration or taking ownership.
func NewRedisStore(client redis.UniversalClient) *RedisStore {
	if client == nil {
		panic("stateful: nil Redis client")
	}
	return &RedisStore{client: client}
}

func (s *RedisStore) prefix(namespace string) string {
	return "core:session:{" + token.Hash(namespace) + "}:"
}
func (s *RedisStore) key(namespace, tokenHash string) string {
	return s.prefix(namespace) + "token:" + tokenHash
}

type record struct {
	Payload string `json:"payload"`
	User    string `json:"user"`
	Epoch   string `json:"epoch"`
	Expires int64  `json:"expires"`
	Idle    int64  `json:"idle"`
}

var create = redis.NewScript(`
if redis.call('EXISTS',KEYS[1])==1 then return 0 end
local epoch=redis.call('GET',KEYS[2]) or ARGV[2]
local t=redis.call('TIME'); local now=t[1]*1000+math.floor(t[2]/1000)
local data=cjson.decode(ARGV[1]); data.epoch=epoch;data.expires=now+tonumber(ARGV[3]);data.idle=0
local ttl=tonumber(ARGV[3]);if tonumber(ARGV[4])>0 then ttl=math.min(ttl,tonumber(ARGV[4]));data.idle=now+ttl end
if redis.call('PTTL',KEYS[2])<tonumber(ARGV[3]) then redis.call('SET',KEYS[2],epoch,'PX',ARGV[3]) end
redis.call('SET',KEYS[1],cjson.encode(data),'PX',ttl);return 1`)

var verify = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return '' end
local ok,data=pcall(cjson.decode,raw);if not ok then return '' end
if data.user~=ARGV[1] or data.epoch~=redis.call('GET',KEYS[2]) then return '' end
local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000)
if type(data.expires)~='number' or type(data.idle)~='number' or data.expires<=now or (data.idle>0 and data.idle<=now) then return '' end
if tonumber(ARGV[2])>0 then
 local ttl=math.min(data.expires-now,tonumber(ARGV[2]));data.idle=now+ttl
 redis.call('SET',KEYS[1],cjson.encode(data),'PX',ttl)
end
return raw`)

func (s *RedisStore) Create(ctx context.Context, req CreateRequest) error {
	data, err := json.Marshal(record{Payload: string(req.Payload), User: req.UserHash})
	if err != nil {
		return err
	}
	epoch, err := token.Random()
	if err != nil {
		return err
	}
	n, err := create.Run(ctx, s.client, []string{s.key(req.Namespace, req.TokenHash), s.prefix(req.Namespace) + "user:" + req.UserHash}, data, epoch, req.AbsoluteTTL.Milliseconds(), req.IdleTTL.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCollision
	}
	return nil
}

func (s *RedisStore) read(ctx context.Context, namespace, tokenHash string, idleTTL time.Duration) ([]byte, error) {
	raw, err := s.client.Get(ctx, s.key(namespace, tokenHash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	var r record
	if json.Unmarshal(raw, &r) != nil || len(r.User) != 64 || r.Epoch == "" || r.Expires <= 0 {
		return nil, ErrInvalid
	}
	result, err := verify.Run(ctx, s.client, []string{s.key(namespace, tokenHash), s.prefix(namespace) + "user:" + r.User}, r.User, idleTTL.Milliseconds()).Text()
	if err != nil {
		return nil, err
	}
	if result == "" || json.Unmarshal([]byte(result), &r) != nil {
		return nil, ErrInvalid
	}
	return []byte(r.Payload), nil
}
func (s *RedisStore) Verify(ctx context.Context, namespace, tokenHash string) ([]byte, error) {
	return s.read(ctx, namespace, tokenHash, 0)
}
func (s *RedisStore) Renew(ctx context.Context, namespace, tokenHash string, idleTTL time.Duration) ([]byte, error) {
	return s.read(ctx, namespace, tokenHash, idleTTL)
}
func (s *RedisStore) Revoke(ctx context.Context, namespace, tokenHash string) error {
	return s.client.Del(ctx, s.key(namespace, tokenHash)).Err()
}
func (s *RedisStore) RevokeAll(ctx context.Context, namespace, userHash string, absoluteTTL time.Duration) error {
	epoch, err := token.Random()
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.prefix(namespace)+"user:"+userHash, epoch, absoluteTTL).Err()
}
