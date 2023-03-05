package waitingroom

import (
	"context"

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
	members, err := a.redisClient.ZRange(ctx, enableDomainKey, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	for _, m := range members {
		e.Logger.Infof("try permit access %v", m)
		site := NewSite(ctx, m, a.config, a.redisClient, a.cache)

		ok, err := site.isEnabledQueue()
		if err != nil {
			return err
		}
		if !ok {
			if err := site.reset(); err != nil {
				return err
			}
			continue
		}
		site.flushPermittedNumberCache()

		if err := site.appendPermitNumberIfGetLock(e); err != nil {
			return err
		}

	}
	return nil
}
