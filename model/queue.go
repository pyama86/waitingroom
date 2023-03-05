package model

import (
	"context"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/pyama86/ngx_waitingroom/waitingroom"
)

type QueueModel struct {
	redisC *redis.Client
	cache  *waitingroom.Cache
	config *waitingroom.Config
}
type Queue struct {
	Domain          string
	CurrentNumber   int64
	PermitetdNumber int64
}

func NewQueueModel(r *redis.Client, config *waitingroom.Config, cache *waitingroom.Cache) *QueueModel {
	return &QueueModel{
		redisC: r,
		cache:  cache,
		config: config,
	}
}
func (q *QueueModel) GetQueues(ctx context.Context, perPage, page int64) ([]Queue, error) {
	domains, err := q.redisC.ZRange(ctx, waitingroom.EnableDomainKey, perPage*(page-1), page*perPage).Result()
	if err != nil {
		return nil, err
	}
	ret := []Queue{}
	for _, domain := range domains {
		cn, err := q.cache.GetAndFetchIfExpired(ctx, domain+waitingroom.SuffixCurrentNo)
		if err != nil {
			return nil, err
		}
		pn, err := q.cache.GetAndFetchIfExpired(ctx, domain+waitingroom.SuffixPermittedNo)
		if err != nil {
			return nil, err
		}

		icn, err := strconv.ParseInt(cn, 10, 64)
		if err != nil {
			return nil, err
		}
		ipn, err := strconv.ParseInt(pn, 10, 64)
		if err != nil {
			return nil, err
		}
		ret = append(ret, Queue{
			CurrentNumber:   icn,
			PermitetdNumber: ipn,
			Domain:          domain,
		})
	}
	return ret, nil
}

func (q *QueueModel) UpdateQueues(ctx context.Context, domain string, m *Queue) error {
	err := q.redisC.Set(ctx, domain+waitingroom.SuffixCurrentNo, m.CurrentNumber, 0).Err()
	if err != nil {
		return err
	}
	err = q.redisC.Set(ctx, domain+waitingroom.SuffixPermittedNo, m.PermitetdNumber, 0).Err()
	if err != nil {
		return err
	}

	return nil
}

func (q *QueueModel) CreateQueues(ctx context.Context, domain string, m *Queue) error {
	site := waitingroom.NewSite(ctx, domain, q.config, q.redisC, q.cache)
	if err := site.EnableQueue(); err != nil {
		return err
	}
	return q.UpdateQueues(ctx, domain, m)
}
func (q *QueueModel) DeleteQueues(ctx context.Context, domain string) error {
	site := waitingroom.NewSite(ctx, domain, q.config, q.redisC, q.cache)
	return site.Reset()
}
