package waitingroom

import (
	"context"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type AccessController struct {
	QueueBase
}

func NewAccessController(config *Config, redisClient *redis.Client) *AccessController {
	return &AccessController{
		QueueBase: QueueBase{
			config:      config,
			redisClient: redisClient,
			cache:       NewCache(redisClient, config),
		},
	}
}
func (a *AccessController) setAllowedNo(ctx context.Context, domain string) (int64, error) {
	// 現在許可されている通り番号
	an, err := a.getAllowedNo(ctx, domain)
	if err != nil && err != redis.Nil {
		return 0, err
	}

	an += a.config.AllowUnitNumber
	_, err = a.redisClient.SetEX(ctx,
		a.allowNoKey(domain),
		strconv.FormatInt(an, 10),
		time.Duration(a.config.QueueEnableSec)*time.Second).Result()
	if err != nil {
		return 0, err
	}
	return an, nil
}

func (a *AccessController) Do(ctx context.Context) error {
	members, err := a.redisClient.SMembers(ctx, enableDomainKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	for _, m := range members {
		exists, err := a.redisClient.Exists(ctx, a.allowNoKey(m)).Result()
		if err != nil {
			return err
		}

		if exists == 0 {
			_, err := a.redisClient.SRem(ctx, enableDomainKey, m).Result()
			if err != nil {
				return err
			}
			continue
		}

		ok, err := a.redisClient.SetNX(ctx, a.lockAllowNoKey(m), "1", time.Duration(a.config.AllowIntervalSec)*time.Second).Result()
		if err != nil {
			return err
		}

		if ok {
			r, err := a.setAllowedNo(ctx, m)
			if err != nil {
				return err
			}
			logrus.Infof("allow access %v over %d", m, r)
		}
	}
	return nil
}
