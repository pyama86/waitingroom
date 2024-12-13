package waitingroom

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/pkg/errors"
	"github.com/pyama86/waitingroom/repository"

	"github.com/nlopes/slack"
)

type Waitingroom struct {
	enableCache              *ttlcache.Cache[string, bool]
	permittedClientCache     *ttlcache.Cache[string, bool]
	currentPermitNumberCache *ttlcache.Cache[string, int64]
	whiteListCache           *ttlcache.Cache[string, bool]
	config                   *Config
	repository               repository.WaitingroomRepositoryer
}

var ErrClientNotIncrese = errors.New("client not increase")

func NewWaitingroom(config *Config, r repository.WaitingroomRepositoryer) *Waitingroom {
	enableCache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](time.Duration(config.CacheTTLSec)*time.Second),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)
	permittedClientCache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](time.Duration(config.CacheTTLSec)*time.Second),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)

	currentPermitNumberCache := ttlcache.New[string, int64](
		ttlcache.WithTTL[string, int64](time.Duration(config.CacheTTLSec)*time.Second),
		ttlcache.WithDisableTouchOnHit[string, int64](),
	)

	whiteListCache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](time.Duration(config.CacheTTLSec)*time.Second),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)

	return &Waitingroom{
		config:                   config,
		enableCache:              enableCache,
		permittedClientCache:     permittedClientCache,
		currentPermitNumberCache: currentPermitNumberCache,
		whiteListCache:           whiteListCache,
		repository:               r,
	}
}

type DomainsParam struct {
	PerPage int64
	Page    int64
}

func (s *Waitingroom) GetEnableDomains(ctx context.Context, params ...*DomainsParam) ([]string, error) {
	page := int64(0)
	perPage := int64(-1)

	if len(params) > 0 {
		page = params[0].Page
		perPage = params[0].PerPage
	}

	return s.repository.GetEnableDomains(ctx, page, perPage)
}

func (s *Waitingroom) GetCurrentNumber(ctx context.Context, domain string) (int64, error) {
	return s.repository.GetCurrentNumber(ctx, domain)
}

func (s *Waitingroom) GetCurrentPermitNumber(ctx context.Context, domain string) (int64, error) {
	return s.repository.GetCurrentPermitNumber(ctx, domain)
}

func (s *Waitingroom) GetEnableDomainsCount(ctx context.Context) (int64, error) {
	return s.repository.GetEnableDomainsCount(ctx)
}

func (s *Waitingroom) AppendPermitNumber(ctx context.Context, domain string) error {
	an, err := s.repository.GetCurrentPermitNumber(ctx, domain)
	if err != nil {
		return errors.Wrap(err, "failed to get current permitted number")
	}

	ttl, err := s.repository.GetCurrentPermitNumberTTL(ctx, domain)
	if err != nil {
		return errors.Wrap(err, "failed to get permitted number ttl")
	}

	cn, err := s.repository.GetCurrentNumber(ctx, domain)
	if err != nil {
		return errors.Wrap(err, "failed to get current number")
	}

	ln, err := s.repository.GetLastNumber(ctx, domain)
	if err != nil {
		return err
	}
	// 前回チェック時より、クライアントが増えていない場合は、即時解除する
	if ln == cn && cn <= an {
		slog.Info(
			"reset waitingroom",
			slog.String("domain", domain),
			slog.Int("current", int(cn)),
			slog.Int("permit", int(an)),
			slog.Int("lastNumber", int(ln)),
			slog.String("ttl", ttl.String()),
		)

		err = s.NotifySlackWithPermittedStatus(domain, "Reset WaitingRoom", ttl, an, cn)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to notify slack: %s", err))
		}

		if err := s.Reset(ctx, domain); err != nil {
			return err
		}
		return ErrClientNotIncrese
	}

	an = an + s.config.PermitUnitNumber

	// 現在のクライアント数が許可数より多いのであれば、起動時間を延長する
	if cn > an {
		ttl = time.Duration(s.config.QueueEnableSec) * time.Second
	}

	if err := s.repository.AppendPermitNumber(ctx, domain, s.config.PermitUnitNumber, ttl); err != nil {
		return err
	}

	if err := s.repository.ExtendCurrentNumberTTL(ctx, domain, ttl); err != nil {
		return err
	}

	if err := s.repository.SaveLastNumber(ctx, domain, cn, ttl); err != nil {
		return err
	}

	slog.Info(
		"append permit number",
		slog.String("domain", domain),
		slog.Int("current", int(cn)),
		slog.Int("permit", int(an)),
		slog.String("ttl", ttl.String()),
	)

	err = s.NotifySlackWithPermittedStatus(domain, "WaitingRoom Additional access granted", ttl, an, cn)
	if err != nil {
		slog.Error(
			"failed to notify slack",
			slog.String("domain", domain),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

func (s *Waitingroom) NotifySlackWithPermittedStatus(domain string, message string, ttl time.Duration, permittedNumber, currentNumber int64) error {

	if currentNumber < 5 {
		slog.Info(
			"skip notify slack",
			slog.String("domain", domain),
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
					{Type: "plain_text", Text: fmt.Sprintf("Domain: %s", domain)},
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

func (s *Waitingroom) flushCache(domain string) {
	s.enableCache.Delete(domain)
	s.currentPermitNumberCache.Delete(domain)

}

func (s *Waitingroom) Reset(ctx context.Context, domain string) error {
	defer s.flushCache(domain)
	return s.repository.DisableDomain(ctx, domain)
}

func (s *Waitingroom) IsInWhitelist(ctx context.Context, domain string) (bool, error) {
	v := s.whiteListCache.Get(domain)
	if v != nil {
		return v.Value(), nil
	}

	r, err := s.repository.IsWhiteListDomain(ctx, domain)
	if err != nil {
		return false, err
	}
	if r {
		s.whiteListCache.Set(domain, r, time.Duration(s.config.CacheTTLSec)*time.Second)
	} else {
		s.whiteListCache.Set(domain, r, time.Duration(s.config.NegativeCacheTTLSec)*time.Second)
	}

	return r, nil
}

func (s *Waitingroom) IsEnabledQueue(ctx context.Context, domain string) (bool, error) {
	num, err := s.currentPermitedNumber(ctx, domain)
	if err != nil {
		return false, err
	}
	return (num >= 0), nil
}

// 制限中ドメインリストに、ロックを取りながらドメインを追加する
func (s *Waitingroom) EnableQueue(ctx context.Context, domain string) error {
	if s.enableCache.Get(domain) == nil {
		if err := s.repository.EnableDomain(ctx, domain, time.Duration(s.config.QueueEnableSec)*time.Second); err != nil {
			return err
		}
		s.flushCache(domain)
		// 大量に更新するとパフォーマンスが落ちるので、TTLの半分の時間は何もしない
		s.enableCache.Set(domain, true, time.Duration(s.config.QueueEnableSec/2)*time.Second)
		slog.Info("EnableQueue", slog.String("enable queue", domain))
	}
	return nil
}

func (s *Waitingroom) IsPermittedClient(ctx context.Context, client *Client) (bool, error) {
	if client.HasID() {
		v := s.permittedClientCache.Get(client.ID)
		if v == nil {
			permitted, err := s.repository.Exists(ctx, client.ID)
			if err != nil {
				return false, err
			}
			ttl := time.Duration(s.config.CacheTTLSec) * time.Second

			if !permitted {
				ttl = time.Duration(s.config.NegativeCacheTTLSec) * time.Second
			}
			s.permittedClientCache.Set(client.ID, permitted, ttl)
			return permitted, nil
		}
		return v.Value(), nil
	}
	return false, nil
}

func (s *Waitingroom) currentPermitedNumber(ctx context.Context, domain string) (int64, error) {
	v := s.currentPermitNumberCache.Get(domain)
	if v != nil {
		return v.Value(), nil
	}

	cn, err := s.repository.GetCurrentPermitNumber(ctx, domain)
	if err != nil {
		return 0, err
	}

	if cn == -1 {
		s.currentPermitNumberCache.Set(domain, 0, time.Duration(s.config.NegativeCacheTTLSec)*time.Second)
		return -1, nil
	}

	s.currentPermitNumberCache.Set(domain, cn, time.Duration(s.config.CacheTTLSec)*time.Second)
	return cn, nil

}

func (s *Waitingroom) CheckAndPermitClient(ctx context.Context, domain string, c *Client) (bool, error) {
	an, err := s.currentPermitedNumber(ctx, domain)
	if err != nil {
		return false, err
	}

	// 許可されたとおり番号以下の値を持っている
	if c.IsPermitClient(an) {
		err := s.repository.PermitClient(ctx, c.ID, time.Duration(s.config.PermittedAccessSec)*time.Second)
		if err != nil {
			return false, err
		}
		slog.Info("PermitClient", slog.String("permit client", c.ID))
		return true, nil
	}
	return false, nil
}

func (s *Waitingroom) AssignSerialNumber(ctx context.Context, domain string, c *Client) (int64, error) {
	if c.HasSerialNumber() {
		return c.SerialNumber, nil
	}

	if !c.HasID() {
		if err := c.AssignID(s.config.EntryDelaySec); err != nil {
			return 0, err
		}
	} else if c.canTakeSerialNumber() {
		cn, err := s.repository.IncrCurrentNumber(ctx, domain, time.Duration(s.config.QueueEnableSec)*time.Second)
		if err != nil {
			return 0, err
		}
		c.AssignSerialNumber(cn)
	}
	return c.SerialNumber, nil
}

func (s *Waitingroom) CalcRemainingWaitSecond(ctx context.Context, domain string, c *Client) (int64, int64, error) {
	remainingWaitSecond := int64(0)
	cp, err := s.currentPermitedNumber(ctx, domain)
	if err != nil {
		return 0, 0, err
	}
	waitDiff := c.SerialNumber - cp
	if waitDiff > 0 {
		if waitDiff%s.config.PermitUnitNumber == 0 {
			remainingWaitSecond = waitDiff / s.config.PermitUnitNumber * int64(s.config.PermitIntervalSec)
		} else {
			remainingWaitSecond = (waitDiff/s.config.PermitUnitNumber + 1) * int64(s.config.PermitIntervalSec)
		}
	}
	return remainingWaitSecond, cp, nil

}

func (s *Waitingroom) ExtendDomainsTTL(ctx context.Context) error {
	return s.repository.ExtendDomainsTTL(ctx, time.Duration(s.config.QueueEnableSec*2)*time.Second)
}

func (s *Waitingroom) SaveCurrentNumber(ctx context.Context, domain string, num int64) error {
	return s.repository.SaveCurrentNumber(ctx, domain, num, time.Duration(s.config.QueueEnableSec)*time.Second)
}

func (s *Waitingroom) SaveCurrentPermitNumber(ctx context.Context, domain string, num int64) error {
	return s.repository.SaveCurrentPermitNumber(ctx, domain, num, time.Duration(s.config.QueueEnableSec)*time.Second)
}

func (s *Waitingroom) GetWhiteListDomains(ctx context.Context, params ...*DomainsParam) ([]string, error) {
	page := int64(0)
	perPage := int64(-1)

	if len(params) > 0 {
		page = params[0].Page
		perPage = params[0].PerPage
	}

	return s.repository.GetWhiteListDomains(ctx, page, perPage)
}

func (s *Waitingroom) AddWhiteListDomain(ctx context.Context, domain string) error {
	return s.repository.AddWhiteListDomain(ctx, domain)
}

func (s *Waitingroom) GetWhiteListDomainsCount(ctx context.Context) (int64, error) {
	return s.repository.GetWhiteListDomainsCount(ctx)
}

func (s *Waitingroom) RemoveWhiteListDomain(ctx context.Context, domain string) error {
	return s.repository.RemoveWhiteListDomain(ctx, domain)
}
