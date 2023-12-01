package waitingroom

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

type AccessController struct {
	cache       *Cache
	redisClient *redis.Client
	config      *Config
}

func NewAccessController(config *Config, redisClient *redis.Client, cache *Cache) *AccessController {
	return &AccessController{
		config:      config,
		redisClient: redisClient,
		cache:       cache,
	}
}
func (a *AccessController) Do(ctx context.Context, e *echo.Echo) error {
	members, err := a.redisClient.ZRange(ctx, EnableDomainKey, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	for _, m := range members {
		e.Logger.Infof("try permit access %v", m)
		site := NewSite(ctx, m, a.config, a.redisClient, a.cache)

		ok, err := site.isEnabledQueue(false)
		if err != nil {
			return err
		}
		if !ok {
			e.Logger.Infof("domain %v is not enabled", m)
			if err := site.Reset(); err != nil {
				return err
			}
			continue
		}
		site.flushCache()

		if err := site.appendPermitNumberIfGetLock(e); err != nil {
			return err
		}

	}

	if len(members) > 0 {
		return a.redisClient.Expire(ctx, EnableDomainKey, time.Duration(a.config.QueueEnableSec*2)*time.Second).Err()
	}

	return nil
}
