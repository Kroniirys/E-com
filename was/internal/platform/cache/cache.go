package cache

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"ecommerce.local/base/internal/platform/metrics"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Cache struct {
	Client *redis.Client
	Prefix string
	slots  chan struct{}
}

func New(client *redis.Client, namespace string) *Cache {
	sum := sha256.Sum256([]byte(namespace))
	return &Cache{client, "ecom:catalog:" + hex.EncodeToString(sum[:12]) + ":", make(chan struct{}, 16)}
}
func Read[T any](ctx context.Context, db *gorm.DB, c *Cache, key string, ttl time.Duration, load func(*gorm.DB) (T, error)) (T, error) {
	var result T
	if c == nil {
		return load(db.WithContext(ctx))
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var version int64
		if e := tx.Table("catalog_generation").Where("id=true").Pluck("version", &version).Error; e != nil {
			return e
		}
		sum := sha256.Sum256([]byte(key))
		name := fmt.Sprintf("%s%d:%x", c.Prefix, version, sum)
		short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		raw, e := c.Client.Get(short, name).Bytes()
		cancel()
		if e == nil && json.Unmarshal(raw, &result) == nil {
			metrics.Add("ecom_cache_hit_total", 1)
			return nil
		}
		if e != nil && e != redis.Nil {
			metrics.Add("ecom_cache_fallback_total", 1)
		} else {
			metrics.Add("ecom_cache_miss_total", 1)
		}
		select {
		case c.slots <- struct{}{}:
			defer func() { <-c.slots }()
		case <-ctx.Done():
			return ctx.Err()
		default:
			return problem.New(503, "DEPENDENCY_UNAVAILABLE", "Catalog is busy; retry later")
		}
		result, e = load(tx)
		if e != nil {
			return e
		}
		raw, e = json.Marshal(result)
		if e != nil {
			return e
		}
		short, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_ = c.Client.Set(short, name, raw, ttl).Err()
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
func (c *Cache) Observe(ctx context.Context, version int64) error {
	short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	return c.Client.Eval(short, `local n=tonumber(redis.call('GET',KEYS[1]) or '0');if tonumber(ARGV[1])>n then redis.call('SET',KEYS[1],ARGV[1]) end;return 1`, []string{c.Prefix + "observed"}, version).Err()
}
