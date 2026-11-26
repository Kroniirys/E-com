package limit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type Limiter interface {
	Allow(context.Context, string, string, int) (bool, int, error)
}
type Redis struct {
	Client *redis.Client
	Prefix string
}

func New(rawURL, prefix string) (*Redis, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL")
	}
	opts.MaxRetries = -1
	opts.DialTimeout = 500 * time.Millisecond
	opts.ReadTimeout = 500 * time.Millisecond
	opts.WriteTimeout = 500 * time.Millisecond
	opts.ContextTimeoutEnabled = true
	return &Redis{redis.NewClient(opts), prefix}, nil
}
func (r *Redis) Allow(ctx context.Context, scope, key string, maximum int) (bool, int, error) {
	hash := sha256.Sum256([]byte(key))
	name := r.Prefix + scope + ":" + hex.EncodeToString(hash[:])
	values, err := r.Client.Eval(ctx, `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('PEXPIRE',KEYS[1],60000) end; return {n,redis.call('PTTL',KEYS[1])}`, []string{name}).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(values) != 2 {
		return false, 0, fmt.Errorf("invalid limiter result")
	}
	count, ok := values[0].(int64)
	if !ok {
		return false, 0, fmt.Errorf("invalid limiter result")
	}
	ttl, ok := values[1].(int64)
	if !ok {
		return false, 0, fmt.Errorf("invalid limiter TTL")
	}
	retry := int((ttl + 999) / 1000)
	if retry < 1 {
		retry = 1
	}
	return count <= int64(maximum), retry, nil
}
