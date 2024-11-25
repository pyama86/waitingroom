package waitingroom

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type QueueModel struct {
	redisC *redis.Client
	config *Config
	cache  *Cache
}
type Queue struct {
	Domain          string `json:"domain" validate:"required,fqdn"`
	CurrentNumber   int64  `json:"current_number" validate:"gte=0"`
	PermitetdNumber int64  `json:"permitted_number" validate:"gte=0"`
}

func NewQueueModel(r *redis.Client, config *Config, cache *Cache) *QueueModel {
	return &QueueModel{
		redisC: r,
		cache:  cache,
		config: config,
	}
}
func (q *QueueModel) GetQueues(ctx context.Context, perPage, page int64) ([]Queue, int64, error) {
	domains, err := q.redisC.ZRange(ctx, EnableDomainKey, perPage*(page-1), page*perPage).Result()
	if err != nil {
		return nil, 0, err
	}
	ret := []Queue{}
	for _, domain := range domains {
		cn, err := q.redisC.Get(ctx, domain+SuffixCurrentNo).Int64()
		if err != nil {
			return nil, 0, err
		}
		pn, err := q.redisC.Get(ctx, domain+SuffixPermittedNo).Int64()
		if err != nil {
			return nil, 0, err
		}

		ret = append(ret, Queue{
			CurrentNumber:   cn,
			PermitetdNumber: pn,
			Domain:          domain,
		})
	}
	total := q.redisC.ZCount(ctx, EnableDomainKey, "-inf", "+inf").Val()
	return ret, total, nil
}

func (q *QueueModel) UpdateQueues(ctx context.Context, m *Queue) error {
	err := q.redisC.Expire(ctx, EnableDomainKey, time.Duration(q.config.QueueEnableSec*2)*time.Second).Err()
	if err != nil {
		return err
	}
	err = q.redisC.SetEX(ctx, m.Domain+SuffixCurrentNo, m.CurrentNumber, time.Duration(q.config.QueueEnableSec)*time.Second).Err()
	if err != nil {
		return err
	}
	err = q.redisC.Set(ctx, m.Domain+SuffixPermittedNo, m.PermitetdNumber, time.Duration(q.config.QueueEnableSec)*time.Second).Err()
	if err != nil {
		return err
	}

	return nil
}

func (q *QueueModel) CreateQueues(ctx context.Context, m *Queue) error {
	site := NewSite(ctx, m.Domain, q.config, q.redisC, q.cache)
	if err := site.EnableQueue(); err != nil {
		return err
	}
	return q.UpdateQueues(ctx, m)
}
func (q *QueueModel) DeleteQueues(ctx context.Context, domain string) error {
	site := NewSite(ctx, domain, q.config, q.redisC, q.cache)
	return site.Reset()
}

type WhiteListModel struct {
	redisC *redis.Client
}

type WhiteList struct {
	Domain string `json:"domain" validate:"required,fqdn"`
}

func NewWhiteListModel(r *redis.Client) *WhiteListModel {
	return &WhiteListModel{
		redisC: r,
	}
}
func (q *WhiteListModel) GetWhiteList(ctx context.Context, perPage, page int64) ([]WhiteList, int64, error) {
	members, err := q.redisC.ZRange(ctx, WhiteListKey, perPage*(page-1), page*perPage).Result()
	if err != nil {
		return nil, 0, err
	}
	ret := []WhiteList{}
	for _, m := range members {
		ret = append(ret, WhiteList{Domain: m})
	}

	total := q.redisC.ZCount(ctx, WhiteListKey, "-inf", "+inf").Val()
	return ret, total, nil
}

func (q *WhiteListModel) CreateWhiteList(ctx context.Context, domain string) error {
	if err := q.redisC.ZAdd(ctx, WhiteListKey, &redis.Z{Score: 1, Member: domain}).Err(); err != nil {
		return err
	}
	return q.redisC.Persist(ctx, WhiteListKey).Err()
}

func (q *WhiteListModel) DeleteWhiteList(ctx context.Context, domain string) error {
	return q.redisC.ZRem(ctx, WhiteListKey, domain).Err()
}
