package repository

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

const suffixPermittedNoLock = "_permitted_no_lock"

type ClusterRepositoryer interface {
	GetLockforPermittedNumber(context.Context, string, time.Duration) (bool, error)
}

type ClusterRepository struct {
	redisC *redis.Client
}

func NewClusterRepository(redisC *redis.Client) *ClusterRepository {
	return &ClusterRepository{
		redisC: redisC,
	}
}

func (c *ClusterRepository) GetLockforPermittedNumber(ctx context.Context, domain string, ttl time.Duration) (bool, error) {
	key := domain + suffixPermittedNoLock
	ok, err := c.redisC.SetNX(ctx, key, "1", 0).Result()
	if err != nil {
		return false, err
	}

	if ok {
		err = c.redisC.Expire(ctx, key, ttl).Err()
		if err != nil {
			return false, err
		}
	}
	return ok, nil
}
