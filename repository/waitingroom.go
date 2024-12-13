package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const suffixPermittedNo = "_permitted_no"
const suffixCurrentNo = "_current_no"
const suffixLastNo = "_last_no"
const enableDomainKey = "queue-domains"
const whiteListKey = "queue-whitelist"

type WaitingroomRepositoryer interface {
	AppendPermitNumber(context.Context, string, int64, time.Duration) error
	SaveLastNumber(context.Context, string, int64, time.Duration) error
	PermitClient(context.Context, string, time.Duration) error
	ExtendCurrentNumberTTL(context.Context, string, time.Duration) error
	GetCurrentPermitNumber(context.Context, string) (int64, error)
	GetCurrentPermitNumberTTL(context.Context, string) (time.Duration, error)
	GetCurrentNumber(context.Context, string) (int64, error)
	GetLastNumber(context.Context, string) (int64, error)
	EnableDomain(context.Context, string, time.Duration) error
	GetEnableDomains(context.Context, int64, int64) ([]string, error)
	GetEnableDomainsCount(context.Context) (int64, error)
	DisableDomain(context.Context, string) error
	ExtendDomainsTTL(context.Context, time.Duration) error
	Exists(context.Context, string) (bool, error)
	IncrCurrentNumber(context.Context, string, time.Duration) (int64, error)
	IsWhiteListDomain(context.Context, string) (bool, error)
	SaveCurrentNumber(context.Context, string, int64, time.Duration) error
	SaveCurrentPermitNumber(context.Context, string, int64, time.Duration) error
	GetWhiteListDomains(context.Context, int64, int64) ([]string, error)
	GetWhiteListDomainsCount(context.Context) (int64, error)
	AddWhiteListDomain(context.Context, string) error
	RemoveWhiteListDomain(context.Context, string) error
}

type WaitingroomRepository struct {
	redisC *redis.Client
}

func NewWaitingroomRepository(redisC *redis.Client) *WaitingroomRepository {
	return &WaitingroomRepository{
		redisC: redisC,
	}
}

func (s *WaitingroomRepository) permittedNumberKey(domain string) string {
	return domain + suffixPermittedNo
}

func (s *WaitingroomRepository) currentNumberKey(domain string) string {
	return domain + suffixCurrentNo
}
func (s *WaitingroomRepository) AppendPermitNumber(ctx context.Context, domain string, appendNum int64, ttl time.Duration) error {
	pipe := s.redisC.Pipeline()
	pipe.IncrBy(ctx, s.permittedNumberKey(domain), appendNum)
	pipe.Expire(ctx,
		s.permittedNumberKey(domain),
		ttl)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("domain: %s, failed to append permit number: %v", domain, err)
	}

	return nil
}

func (s *WaitingroomRepository) GetCurrentPermitNumber(ctx context.Context, domain string) (int64, error) {
	v, err := s.redisC.Get(ctx, s.permittedNumberKey(domain)).Int64()
	if err != nil {
		if err == redis.Nil {
			return -1, nil
		}
		return 0, err
	}
	return v, err
}

func (s *WaitingroomRepository) GetCurrentPermitNumberTTL(ctx context.Context, domain string) (time.Duration, error) {
	return s.redisC.TTL(ctx, s.permittedNumberKey(domain)).Result()
}

func (s *WaitingroomRepository) GetCurrentNumber(ctx context.Context, domain string) (int64, error) {
	return s.redisC.Get(ctx, s.currentNumberKey(domain)).Int64()
}

func (s *WaitingroomRepository) lastNumberKey(domain string) string {
	return domain + suffixLastNo
}
func (s *WaitingroomRepository) GetLastNumber(ctx context.Context, domain string) (int64, error) {
	v, err := s.redisC.Get(ctx, s.lastNumberKey(domain)).Int64()
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("failed to get last number %s:%v", domain, err)
	}
	return v, nil
}

func (s *WaitingroomRepository) SaveLastNumber(ctx context.Context, domain string, lastNum int64, ttl time.Duration) error {
	return s.redisC.SetEX(ctx, s.lastNumberKey(domain), lastNum, ttl).Err()
}
func (s *WaitingroomRepository) PermitClient(ctx context.Context, clientID string, ttl time.Duration) error {
	return s.redisC.SetEX(ctx, clientID, 1, ttl).Err()
}

func (s *WaitingroomRepository) ExtendCurrentNumberTTL(ctx context.Context, domain string, ttl time.Duration) error {
	return s.redisC.Expire(ctx, s.currentNumberKey(domain), ttl).Err()
}

func (s *WaitingroomRepository) GetEnableDomains(ctx context.Context, page, perPage int64) ([]string, error) {
	v, err := s.redisC.ZRange(ctx, enableDomainKey, page, perPage).Result()
	if err != nil {
		if err == redis.Nil {
			return []string{}, nil
		}
		return nil, err
	}
	return v, err
}

func (s *WaitingroomRepository) GetEnableDomainsCount(ctx context.Context) (int64, error) {
	return s.redisC.ZCount(ctx, enableDomainKey, "-inf", "+inf").Result()
}

func (s *WaitingroomRepository) IsWhiteListDomain(ctx context.Context, domain string) (bool, error) {
	v, _, err := s.redisC.ZScan(ctx, whiteListKey, 0, domain, 1).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}

	return len(v) > 0, err
}

func (s *WaitingroomRepository) DisableDomain(ctx context.Context, domain string) error {
	pipe := s.redisC.Pipeline()
	pipe.ZRem(ctx, enableDomainKey, domain)
	pipe.Del(ctx, s.currentNumberKey(domain),
		s.permittedNumberKey(domain),
		s.lastNumberKey(domain))
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return err
	}
	return nil
}
func (s *WaitingroomRepository) ExtendDomainsTTL(ctx context.Context, ttl time.Duration) error {
	return s.redisC.Expire(ctx, enableDomainKey, ttl).Err()
}

func (s *WaitingroomRepository) EnableDomain(ctx context.Context, domain string, ttl time.Duration) error {
	pipe := s.redisC.Pipeline()
	// 値があれば上書きしない、なければ作る
	pipe.SetNX(ctx, s.permittedNumberKey(domain), "0", 0)
	pipe.Expire(ctx, s.permittedNumberKey(domain), ttl)
	pipe.ZAdd(ctx, enableDomainKey, &redis.Z{
		Score:  1,
		Member: domain,
	})
	pipe.Expire(ctx, enableDomainKey, ttl*2)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *WaitingroomRepository) Exists(ctx context.Context, key string) (bool, error) {
	v, err := s.redisC.Exists(ctx, key).Uint64()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return v > 0, err
}

func (s *WaitingroomRepository) IncrCurrentNumber(ctx context.Context, domain string, ttl time.Duration) (int64, error) {
	pipe := s.redisC.Pipeline()
	incr := pipe.Incr(ctx, s.currentNumberKey(domain))
	pipe.Expire(ctx,
		s.currentNumberKey(domain), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func (s *WaitingroomRepository) SaveCurrentNumber(ctx context.Context, domain string, num int64, ttl time.Duration) error {
	return s.redisC.SetEX(ctx, s.currentNumberKey(domain), num, ttl).Err()
}

func (s *WaitingroomRepository) SaveCurrentPermitNumber(ctx context.Context, domain string, num int64, ttl time.Duration) error {
	return s.redisC.SetEX(ctx, s.permittedNumberKey(domain), num, ttl).Err()
}

func (s *WaitingroomRepository) GetWhiteListDomains(ctx context.Context, page, perPage int64) ([]string, error) {
	return s.redisC.ZRange(ctx, whiteListKey, page, perPage).Result()
}

func (s *WaitingroomRepository) GetWhiteListDomainsCount(ctx context.Context) (int64, error) {
	return s.redisC.ZCount(ctx, whiteListKey, "-inf", "+inf").Result()
}

func (s *WaitingroomRepository) AddWhiteListDomain(ctx context.Context, domain string) error {
	if err := s.redisC.ZAdd(ctx, whiteListKey, &redis.Z{Score: 1, Member: domain}).Err(); err != nil {
		return err
	}
	return s.redisC.Persist(ctx, whiteListKey).Err()
}

func (s *WaitingroomRepository) RemoveWhiteListDomain(ctx context.Context, domain string) error {
	return s.redisC.ZRem(ctx, whiteListKey, domain).Err()
}
