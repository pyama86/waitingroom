package waitingroom

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

type Site struct {
	domain                       string
	ctx                          context.Context
	redisC                       *redis.Client
	cache                        *Cache
	config                       *Config
	permittedNumberKey           string
	currentNumberKey             string
	appendPermittedNumberLockKey string
}

const enableDomainKey = "queue-domains"

func NewSite(c context.Context, domain string, config *Config, r *redis.Client, cache *Cache) *Site {
	return &Site{
		domain:                       domain,
		ctx:                          c,
		redisC:                       r,
		cache:                        cache,
		config:                       config,
		permittedNumberKey:           domain + "_permitted_no",
		currentNumberKey:             domain + "_current_no",
		appendPermittedNumberLockKey: domain + "_permitted_no_lock",
	}
}

func (s *Site) appendPermitNumber(e *echo.Echo) error {
	an, err := s.currentPermitedNumber(false)
	if err != nil && err != redis.Nil {
		return err
	}

	ttl, err := s.redisC.TTL(s.ctx, s.currentNumberKey).Result()
	if err != nil {
		return err
	}

	an = an + s.config.PermitUnitNumber
	err = s.redisC.SetEX(s.ctx,
		s.permittedNumberKey,
		strconv.FormatInt(an, 10),
		ttl).Err()
	if err != nil {
		return fmt.Errorf("domain: %s value: %d ttl: %d, err: %s", s.domain, an, ttl/time.Second, err)
	}

	e.Logger.Infof("domain: %s value: %d ttl: %d, permit: %d", s.domain, an, ttl/time.Second, an)
	return nil
}

func (s *Site) appendPermitNumberIfGetLock(e *echo.Echo) error {
	// 古いサーバだとSetNXにTTLを渡せない
	ok, err := s.redisC.SetNX(s.ctx, s.appendPermittedNumberLockKey, "1", 0).Result()
	if err != nil {
		return err
	}

	if ok {
		e.Logger.Infof("got lock %v", s.domain)
		err = s.redisC.Expire(s.ctx, s.appendPermittedNumberLockKey, time.Duration(s.config.PermitIntervalSec)*time.Second).Err()
		if err != nil {
			return err
		}

		if err := s.appendPermitNumber(e); err != nil {
			return err
		}
	}
	return nil
}
func (s *Site) flushPermittedNumberCache() {
	s.cache.Delete(s.permittedNumberKey)
}

func (s *Site) reset() error {
	pipe := s.redisC.Pipeline()
	pipe.SRem(s.ctx, enableDomainKey, s.domain)
	pipe.Del(s.ctx, s.currentNumberKey, s.permittedNumberKey)
	_, err := pipe.Exec(s.ctx)
	if err != nil && err != redis.Nil {
		return err
	}
	return nil
}

func (s *Site) isEnabledQueue() (bool, error) {
	num, err := s.redisC.Exists(s.ctx, s.permittedNumberKey).Uint64()
	if err != nil {
		return false, err
	}
	return (num > 0), nil
}

func (s *Site) enableQueueIfWant(c echo.Context) error {
	cacheKey := s.permittedNumberKey + "_enable_cache"

	if c.Param("enable") != "" && !s.cache.Exists(cacheKey) {
		pipe := s.redisC.Pipeline()
		// 値があれば上書きしない、なければ作る
		pipe.SetNX(s.ctx, s.permittedNumberKey, "0", 0)
		pipe.Expire(s.ctx, s.permittedNumberKey, time.Duration(s.config.QueueEnableSec)*time.Second)
		pipe.SAdd(s.ctx, enableDomainKey, s.domain)
		_, err := pipe.Exec(s.ctx)
		if err != nil {
			return err
		}

		// 大量に更新するとパフォーマンスが落ちるので、TTLの半分の時間は何もしない
		s.cache.Set(cacheKey, "1", time.Duration(s.config.QueueEnableSec/2)*time.Second)
	}
	return nil
}

func (s *Site) isPermitClient(client *Client) bool {
	cacheKey := s.permittedNumberKey + "_disable_cache"
	if s.cache.Exists(cacheKey) {
		return true
	}

	// ドメインでqueueが有効ではないので制限されていない
	if _, err := s.cache.GetAndFetchIfExpired(
		s.ctx,
		s.permittedNumberKey); err == redis.Nil {

		s.cache.Set(cacheKey, "1", time.Duration(s.config.NegativeCacheTTLSec)*time.Second)
		return true
	}

	// 許可済みのコネクション
	if client.ID != "" && client.SerialNumber != 0 {
		_, err := s.cache.GetAndFetchIfExpired(s.ctx, client.ID)
		if err == nil {
			return true
		}
	}
	return false
}

func (s *Site) IncrCurrentNumber() (int64, error) {
	pipe := s.redisC.Pipeline()
	incr := pipe.Incr(s.ctx, s.currentNumberKey)
	pipe.Expire(s.ctx,
		s.currentNumberKey, time.Duration(s.config.QueueEnableSec)*time.Second)
	if _, err := pipe.Exec(s.ctx); err != nil {
		return 0, nil
	}
	return incr.Val(), nil
}

func (s *Site) currentPermitedNumber(useCache bool) (int64, error) {
	// 現在許可されている通り番号
	if useCache {
		v, err := s.cache.GetAndFetchIfExpired(s.ctx, s.permittedNumberKey)
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(v, 10, 64)
	}
	v, err := s.redisC.Get(s.ctx, s.permittedNumberKey).Int64()
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *Site) CanClientAccess(c *Client) (bool, error) {
	an, err := s.currentPermitedNumber(true)
	if err != nil {
		return false, err
	}

	// 許可されたとおり番号以下の値を持っている
	if c.SerialNumber != 0 && an >= c.SerialNumber {
		err := s.redisC.SetEX(s.ctx, c.ID, strconv.FormatInt(c.SerialNumber, 10),
			time.Duration(s.config.PermittedAccessSec)*time.Second,
		).Err()

		if err != nil {
			return false, err
		}

		return true, nil
	}
	return false, nil
}
