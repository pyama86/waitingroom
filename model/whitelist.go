package model

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/pyama86/ngx_waitingroom/waitingroom"
)

type WhiteListModel struct {
	redisC *redis.Client
	cache  *waitingroom.Cache
	config *waitingroom.Config
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
	members, err := q.redisC.ZRange(ctx, waitingroom.WhiteListKey, perPage*(page-1), page*perPage).Result()
	if err != nil {
		return nil, 0, err
	}
	ret := []WhiteList{}
	for _, m := range members {
		ret = append(ret, WhiteList{Domain: m})
	}

	total := q.redisC.ZCount(ctx, waitingroom.WhiteListKey, "-inf", "+inf").Val()
	return ret, total, nil
}

func (q *WhiteListModel) CreateWhiteList(ctx context.Context, domain string) error {
	if err := q.redisC.ZAdd(ctx, waitingroom.WhiteListKey, &redis.Z{Score: 1, Member: domain}).Err(); err != nil {
		return err
	}
	return q.redisC.Persist(ctx, waitingroom.WhiteListKey).Err()
}

func (q *WhiteListModel) DeleteWhiteList(ctx context.Context, domain string) error {
	return q.redisC.ZRem(ctx, waitingroom.WhiteListKey, domain).Err()
}
