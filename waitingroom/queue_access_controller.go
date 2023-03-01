package waitingroom

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

type AccessController struct {
	QueueBase
}

func NewAccessController(config *Config, redisClient *redis.Client, cache *Cache) *AccessController {
	return &AccessController{
		QueueBase: QueueBase{
			config:      config,
			redisClient: redisClient,
			cache:       cache,
		},
	}
}
func (a *AccessController) setAllowedNo(ctx context.Context, domain string) (int64, error) {
	// 現在許可されている通り番号
	an, err := a.getAllowedNo(ctx, domain, false)
	if err != nil && err != redis.Nil {
		return 0, err
	}

	an = an + a.config.AllowUnitNumber
	_, err = a.redisClient.Set(ctx,
		a.allowNoKey(domain),
		strconv.FormatInt(an, 10),
		redis.KeepTTL).Result()
	if err != nil {
		return 0, err
	}

	a.cache.Delete(a.allowNoKey(domain))

	return an, nil
}

func (a *AccessController) Do(ctx context.Context, e *echo.Echo) error {
	members, err := a.redisClient.SMembers(ctx, enableDomainKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	for _, m := range members {
		e.Logger.Infof("try allow access %v", m)
		exists, err := a.redisClient.Exists(ctx, a.allowNoKey(m)).Result()
		if err != nil {
			return err
		}

		e.Logger.Infof("allow no keys %v", exists)
		if exists == 0 {
			_, err := a.redisClient.SRem(ctx, enableDomainKey, m).Result()
			if err != nil {
				return err
			}
			_, err = a.redisClient.Del(ctx, a.allowNoKey(m), m).Result()
			if err != nil && err != redis.Nil {
				return err
			}

			continue
		}

		ok, err := a.redisClient.SetNX(ctx, a.lockAllowNoKey(m), "1", time.Duration(a.config.AllowIntervalSec)*time.Second).Result()
		if err != nil {
			return err
		}

		if ok {
			e.Logger.Infof("got lock %v %s", m, os.Hostname())
			_, err := a.redisClient.Expire(ctx, a.lockAllowNoKey(m), time.Duration(a.config.AllowIntervalSec)*time.Second).Result()
			if err != nil {
				return err
			}
			r, err := a.setAllowedNo(ctx, m)
			if err != nil {
				return err
			}

			e.Logger.Infof("allow access %v over %d", m, r)
		} else {
			ttl, err := a.redisClient.TTL(ctx, a.lockAllowNoKey(m)).Result()
			if err != nil {
				return err
			}
			e.Logger.Infof("%v can't get lock ttl: %d sec", m, ttl/time.Second)
		}
	}
	return nil
}
