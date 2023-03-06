package model

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pyama86/ngx_waitingroom/waitingroom"
)

type QueueModel struct {
	redisC *redis.Client
	config *waitingroom.Config
	cache  *waitingroom.Cache
}
type Queue struct {
	Domain          string `json:"domain" validate:"required,fqdn"`
	CurrentNumber   int64  `json:"current_number" validate:"gte=0"`
	PermitetdNumber int64  `json:"permitted_number" validate:"gte=0"`
}

func NewQueueModel(r *redis.Client, config *waitingroom.Config, cache *waitingroom.Cache) *QueueModel {
	return &QueueModel{
		redisC: r,
		cache:  cache,
		config: config,
	}
}
func (q *QueueModel) GetQueues(ctx context.Context, perPage, page int64) ([]Queue, int64, error) {
	domains, err := q.redisC.ZRange(ctx, waitingroom.EnableDomainKey, perPage*(page-1), page*perPage).Result()
	if err != nil {
		return nil, 0, err
	}
	ret := []Queue{}
	for _, domain := range domains {
		cn, err := q.redisC.Get(ctx, domain+waitingroom.SuffixCurrentNo).Int64()
		if err != nil {
			return nil, 0, err
		}
		pn, err := q.redisC.Get(ctx, domain+waitingroom.SuffixPermittedNo).Int64()
		if err != nil {
			return nil, 0, err
		}

		ret = append(ret, Queue{
			CurrentNumber:   cn,
			PermitetdNumber: pn,
			Domain:          domain,
		})
	}
	total := q.redisC.ZCount(ctx, waitingroom.EnableDomainKey, "-inf", "+inf").Val()
	return ret, total, nil
}

func (q *QueueModel) UpdateQueues(ctx context.Context, m *Queue) error {
	err := q.redisC.Expire(ctx, waitingroom.EnableDomainKey, time.Duration(q.config.QueueEnableSec*2)*time.Second).Err()
	if err != nil {
		return err
	}
	err = q.redisC.SetEX(ctx, m.Domain+waitingroom.SuffixCurrentNo, m.CurrentNumber, time.Duration(q.config.QueueEnableSec)*time.Second).Err()
	if err != nil {
		return err
	}
	err = q.redisC.Set(ctx, m.Domain+waitingroom.SuffixPermittedNo, m.PermitetdNumber, time.Duration(q.config.QueueEnableSec)*time.Second).Err()
	if err != nil {
		return err
	}

	return nil
}

func (q *QueueModel) CreateQueues(ctx context.Context, m *Queue) error {
	site := waitingroom.NewSite(ctx, m.Domain, q.config, q.redisC, q.cache)
	if err := site.EnableQueue(); err != nil {
		return err
	}
	return q.UpdateQueues(ctx, m)
}
func (q *QueueModel) DeleteQueues(ctx context.Context, domain string) error {
	site := waitingroom.NewSite(ctx, domain, q.config, q.redisC, q.cache)
	return site.Reset()
}
