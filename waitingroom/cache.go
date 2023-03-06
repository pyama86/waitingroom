package waitingroom

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
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

func (c *Cache) Delete(key string) {
	c.cache.Delete(key)
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.cache.Set(key, value, ttl)
}

func (c *Cache) Exists(key string) bool {
	_, ok := c.cache.Get(key)
	return ok
}

func (c *Cache) GetAndFetchIfExpired(ctx context.Context, key string) (string, error) {
	v, found := c.cache.Get(key)
	if found {
		return v.(string), nil
	}

	rv, err := c.redisClient.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	c.cache.Set(key, rv, cache.DefaultExpiration)
	return rv, nil
}

func (c *Cache) ZScanAndFetchIfExpired(ctx context.Context, key, target string) ([]string, error) {
	cacheKey := key + target
	v, found := c.cache.Get(cacheKey)
	if found {
		return v.([]string), nil
	}

	rv, _, err := c.redisClient.ZScan(ctx, key, 0, target, 1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	c.cache.Set(cacheKey, rv, cache.DefaultExpiration)
	return rv, nil
}
