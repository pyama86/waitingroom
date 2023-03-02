package waitingroom

import (
	"context"
	"fmt"
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
func (a *AccessController) setAllowedNo(ctx context.Context, domain string) (int64, int64, error) {
	// 現在許可されている通り番号
	an, err := a.getAllowedNo(ctx, domain, false)
	if err != nil && err != redis.Nil {
		return 0, 0, err
	}

	ttl, err := a.redisClient.TTL(ctx, a.allowNoKey(domain)).Result()
	if err != nil {
		return 0, 0, err
	}

	an = an + a.config.AllowUnitNumber
	err = a.redisClient.SetEX(ctx,
		a.allowNoKey(domain),
		strconv.FormatInt(an, 10),
		ttl).Err()
	if err != nil {
		return 0, 0, fmt.Errorf("domain: %s value: %d ttl: %d, err:: %s", domain, an, ttl/time.Second, err)
	}

	return an, int64(ttl / time.Second), nil
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

		if exists == 0 {
			pipe := a.redisClient.Pipeline()
			pipe.SRem(ctx, enableDomainKey, m)
			pipe.Del(ctx, a.hostCurrentNumberKey(m), a.allowNoKey(m))
			_, err := pipe.Exec(ctx)
			if err != nil && err != redis.Nil {
				return err
			}
			continue
		}
		// キャッシュ削除
		a.cache.Delete(a.allowNoKey(m))

		// 古いサーバだとSetNXにTTLを渡せない
		ok, err := a.redisClient.SetNX(ctx, a.lockAllowNoKey(m), "1", 0).Result()
		if err != nil {
			e.Logger.Warnf("can't set nx %s %s", m, err)
			return err
		}

		if ok {
			e.Logger.Infof("got lock %v", m)
			err = a.redisClient.Expire(ctx, a.lockAllowNoKey(m), time.Duration(a.config.AllowIntervalSec)*time.Second).Err()
			if err != nil {
				return err
			}
			r, ttl, err := a.setAllowedNo(ctx, m)
			if err != nil {
				e.Logger.Warnf("can't set allowed no %s err:%s", m, err)
				return err
			}

			e.Logger.Infof("allow access %v over %d ttl %d sec", m, r, ttl)
		}
	}
	return nil
}
