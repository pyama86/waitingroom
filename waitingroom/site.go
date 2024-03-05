package waitingroom

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/nlopes/slack"
)

type Site struct {
	domain                       string
	ctx                          context.Context
	redisC                       *redis.Client
	cache                        *Cache
	config                       *Config
	permittedNumberKey           string // 何番目まで許可されているかの番号
	currentNumberKey             string // 現在の発券番号
	lastNumberKey                string // 最後にチェックしたときの発券番号
	appendPermittedNumberLockKey string // 許可番号を更新する際のロックキー
	cacheEnableKey               string // 最後に有効にしてからの処理遅延のキャッシュ
}

var ErrClientNotIncrese = errors.New("client not increase")

// 制限中のドメインリスト
const EnableDomainKey = "queue-domains"
const WhiteListKey = "queue-whitelist"

const SuffixPermittedNo = "_permitted_no"
const SuffixCurrentNo = "_current_no"
const SuffixLastNo = "_last_no"
const SuffixPermittedNoLock = "_permitted_no_lock"
const SuffixCacheEnable = "_enable_cache"

func NewSite(c context.Context, domain string, config *Config, r *redis.Client, cache *Cache) *Site {
	return &Site{
		domain:                       domain,
		ctx:                          c,
		redisC:                       r,
		cache:                        cache,
		config:                       config,
		permittedNumberKey:           domain + SuffixPermittedNo,
		currentNumberKey:             domain + SuffixCurrentNo,
		lastNumberKey:                domain + SuffixLastNo,
		appendPermittedNumberLockKey: domain + SuffixPermittedNoLock,
		cacheEnableKey:               domain + SuffixCacheEnable,
	}
}

func (s *Site) appendPermitNumber(e *echo.Echo) error {
	an, err := s.currentPermitedNumber(false)
	if err != nil {
		if err != redis.Nil {
			return fmt.Errorf("append permit number get current permitted failed: %s", err)
		}
		return err
	}

	ttl, err := s.redisC.TTL(s.ctx, s.permittedNumberKey).Result()
	if err != nil {
		return err
	}

	cn, err := s.redisC.Get(s.ctx, s.currentNumberKey).Int64()
	if err != nil {
		if err != redis.Nil {
			return err
		}
		cn = 0
	}

	ln, err := s.redisC.Get(s.ctx, s.lastNumberKey).Int64()
	if err != nil {
		if err != redis.Nil {
			return err
		}
		ln = 0
	}
	// 前回チェック時より、クライアントが増えていない場合は、即時解除する
	if ln == cn && cn <= an {
		slog.Info(
			"reset waitingroom",
			slog.String("domain", s.domain),
			slog.Int("current", int(cn)),
			slog.Int("permit", int(an)),
			slog.Int("lastNumber", int(ln)),
			slog.String("ttl", ttl.String()),
		)

		err = s.notifySlackWithPermittedStatus(e, "Reset WaitingRoom", ttl, an, cn)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to notify slack: %s", err))
		}

		if err := s.Reset(); err != nil {
			return err
		}
		return ErrClientNotIncrese
	}

	an = an + s.config.PermitUnitNumber

	// 現在のクライアント数が許可数より多いのであれば、起動時間を延長する
	if cn > an {
		ttl = time.Duration(s.config.QueueEnableSec) * time.Second
	}

	pipe := s.redisC.Pipeline()
	pipe.SetEX(s.ctx,
		s.permittedNumberKey,
		strconv.FormatInt(an, 10),
		ttl)

	pipe.Expire(s.ctx,
		s.currentNumberKey,
		ttl)

	pipe.SetEX(s.ctx,
		s.lastNumberKey,
		strconv.FormatInt(cn, 10),
		ttl,
	)
	_, err = pipe.Exec(s.ctx)

	if err != nil && err != redis.Nil {
		return fmt.Errorf("domain: %s current: %d ttl: %d permit: %d, err: %s", s.domain, cn, an, ttl/time.Second, err)
	}

	slog.Info(
		"append permit number",
		slog.String("domain", s.domain),
		slog.Int("current", int(cn)),
		slog.Int("permit", int(an)),
		slog.String("ttl", ttl.String()),
	)

	err = s.notifySlackWithPermittedStatus(e, "WaitingRoom Additional access granted", ttl, an, cn)
	if err != nil {
		slog.Error(
			"failed to notify slack",
			slog.String("domain", s.domain),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

func (s *Site) notifySlackWithPermittedStatus(e *echo.Echo, message string, ttl time.Duration, permittedNumber, currentNumber int64) error {

	if currentNumber < 5 {
		slog.Info(
			"skip notify slack",
			slog.String("domain", s.domain),
			slog.Int64("current", currentNumber),
			slog.Int64("permit", permittedNumber),
			slog.String("ttl", ttl.String()),
		)
		return nil
	}

	if s.config.SlackApiToken != "" && s.config.SlackChannel != "" {
		c := slack.New(s.config.SlackApiToken)
		_, _, err := c.PostMessage(s.config.SlackChannel, slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				&slack.TextBlockObject{Type: "mrkdwn", Text: fmt.Sprintf("*%s*", message)},
				[]*slack.TextBlockObject{
					{Type: "plain_text", Text: fmt.Sprintf("Domain: %s", s.domain)},
					{Type: "plain_text", Text: fmt.Sprintf("CurrentClient: %d", currentNumber)},
					{Type: "plain_text", Text: fmt.Sprintf("PermittedNumber: %d", permittedNumber)},
					{Type: "plain_text", Text: fmt.Sprintf("TTL: %d", ttl/time.Second)},
					{Type: "plain_text", Text: fmt.Sprintf("Time: %s", time.Now().Format("2006-01-02 15:04:05"))},
				},
				nil,
			),
		))
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Site) appendPermitNumberIfGetLock(e *echo.Echo) error {
	// 古いサーバだとSetNXにTTLを渡せない
	ok, err := s.redisC.SetNX(s.ctx, s.appendPermittedNumberLockKey, "1", 0).Result()
	if err != nil {
		return err
	}

	if ok {
		slog.Info(
			"got lock",
			slog.String("domain", s.domain),
		)
		err = s.redisC.Expire(s.ctx, s.appendPermittedNumberLockKey, time.Duration(s.config.PermitIntervalSec)*time.Second).Err()
		if err != nil {
			return fmt.Errorf("failed to set expire %s:%v", s.domain, err)
		}

		if err := s.appendPermitNumber(e); err != nil {
			if errors.Is(err, ErrClientNotIncrese) {
				slog.Info("client not increase", slog.String("domain", s.domain))
				return nil
			}
			return fmt.Errorf("failed to append permit number %s:%v", s.domain, err)
		}
	}
	return nil
}

func (s *Site) flushCache() {
	s.cache.Delete(s.permittedNumberKey)
	s.cache.Delete(s.cacheEnableKey)
}

func (s *Site) Reset() error {
	defer s.flushCache()
	pipe := s.redisC.Pipeline()
	pipe.ZRem(s.ctx, EnableDomainKey, s.domain)
	pipe.Del(s.ctx, s.currentNumberKey, s.permittedNumberKey, s.appendPermittedNumberLockKey, s.lastNumberKey)
	_, err := pipe.Exec(s.ctx)
	if err != nil && err != redis.Nil {
		return err
	}
	return nil
}

func (s *Site) isInWhitelist() (bool, error) {
	val, err := s.cache.ZScanAndFetchIfExpired(s.ctx, WhiteListKey, s.domain)
	if len(val) != 0 {
		return true, nil
	}
	return false, err
}

func (s *Site) isEnabledQueue(cache bool) (bool, error) {
	if cache {
		v, err := s.currentPermitedNumber(true)
		if err != nil && err == redis.Nil {
			return false, nil
		}

		return v >= 0, err
	} else {
		num, err := s.redisC.Exists(s.ctx, s.permittedNumberKey).Uint64()
		if err != nil {
			if err == redis.Nil {
				return false, nil
			}
			return false, err
		}
		return (num > 0), nil
	}
}

// 制限中ドメインリストに、ロックを取りながらドメインを追加する
func (s *Site) EnableQueue() error {
	if !s.cache.Exists(s.cacheEnableKey) {
		pipe := s.redisC.Pipeline()
		// 値があれば上書きしない、なければ作る
		pipe.SetNX(s.ctx, s.permittedNumberKey, "0", 0)
		pipe.Expire(s.ctx, s.permittedNumberKey, time.Duration(s.config.QueueEnableSec)*time.Second)
		pipe.ZAdd(s.ctx, EnableDomainKey, &redis.Z{
			Score:  1,
			Member: s.domain,
		})
		pipe.Expire(s.ctx, EnableDomainKey, time.Duration(s.config.QueueEnableSec*2)*time.Second)
		_, err := pipe.Exec(s.ctx)
		if err != nil {
			return err
		}
		s.flushCache()
		// 大量に更新するとパフォーマンスが落ちるので、TTLの半分の時間は何もしない
		s.cache.Set(s.cacheEnableKey, "1", time.Duration(s.config.QueueEnableSec/2)*time.Second)
		slog.Info("EnableQueue", slog.String("enable queue", s.domain))
	}
	return nil
}

func (s *Site) isPermittedClient(client *Client) (bool, error) {
	// 許可済みのコネクション
	if client.ID != "" {
		v, err := s.cache.GetAndFetchIfExpired(s.ctx, client.ID)
		if err != nil {
			if err == redis.Nil {
				s.cache.Set(client.ID, "-1", time.Duration(s.config.NegativeCacheTTLSec)*time.Second)
				return false, nil
			}
			return false, err
		}
		return v != "-1", nil
	}
	return false, nil
}

func (s *Site) incrCurrentNumber() (int64, error) {
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
			if err == redis.Nil {
				// ドメインでqueueが有効ではないので制限されていない
				s.cache.Set(s.permittedNumberKey, "-1", time.Duration(s.config.NegativeCacheTTLSec)*time.Second)
			}
			return 0, err
		}
		if v == "-1" {
			return 0, redis.Nil
		}
		return strconv.ParseInt(v, 10, 64)
	}

	v, err := s.redisC.Get(s.ctx, s.permittedNumberKey).Int64()
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *Site) permitClient(c *Client) (bool, error) {
	an, err := s.currentPermitedNumber(true)
	if err != nil {
		if err == redis.Nil {
			return true, nil
		}
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

		slog.Info("PermitClient", slog.String("permit client", c.ID))
		return true, nil
	}
	return false, nil
}
