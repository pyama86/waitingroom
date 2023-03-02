package waitingroom

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

const waitingInfoCookieKey = "waiting-room"
const paramDomainKey = "domain"
const enableDomainKey = "queue-domains"

type QueueBase struct {
	sc          *securecookie.SecureCookie
	cache       *Cache
	redisClient *redis.Client
	config      *Config
}

func (q *QueueBase) setWaitingInfoCookie(c echo.Context, waitingInfo *WaitingInfo) error {
	encoded, err := q.sc.Encode(waitingInfoCookieKey, waitingInfo)
	if err != nil {
		return err
	}

	c.SetCookie(&http.Cookie{
		Name:     waitingInfoCookieKey,
		Value:    encoded,
		MaxAge:   q.config.AllowedAccessSec,
		Domain:   c.Param(paramDomainKey),
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
	return nil
}

func (q *QueueBase) hostCurrentNumberKey(domain string) string {
	return domain + "_current_no"
}

func (q *QueueBase) allowNoKey(domain string) string {
	return domain + "_allow_no"
}

func (q *QueueBase) lockAllowNoKey(domain string) string {
	return domain + "_lock_allow_no"
}

func (q *QueueBase) getAllowedNo(ctx context.Context, domain string, usecache bool) (int64, error) {
	// 現在許可されている通り番号
	if usecache {
		v, err := q.cache.GetAndFetchIfExpired(ctx, q.allowNoKey(domain))
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(v, 10, 64)
	} else {
		v, err := q.redisClient.Get(ctx, q.allowNoKey(domain)).Int64()
		if err != nil {
			return 0, err
		}
		return v, nil
	}
}
