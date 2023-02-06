package waitingroom

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	gocache "github.com/patrickmn/go-cache"
)

type Cache struct {
	cache       *gocache.Cache
	redisClient *redis.Client
}

func NewCache(redisClient *redis.Client, config *Config) *Cache {
	return &Cache{
		cache: gocache.New(time.Duration(config.CacheTTLSec)*time.Second,
			time.Duration(config.CacheTTLSec*2)*time.Second),
		redisClient: redisClient,
	}
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	v, found := c.cache.Get(key)
	if found {
		return v.(string), nil
	}

	rv := c.redisClient.Get(ctx, key)
	if rv.Err() != nil {
		return "", rv.Err()
	}

	c.cache.Set(key, rv.Val(), gocache.DefaultExpiration)
	return rv.Val(), nil
}
