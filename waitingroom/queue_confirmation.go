package waitingroom

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type QueueConfirmation struct {
	QueueBase
	cache       *Cache
	redisClient *redis.Client
}

func NewQueueConfirmation(
	sc *securecookie.SecureCookie,
	config *Config,
	redisClient *redis.Client,
) *QueueConfirmation {
	return &QueueConfirmation{
		QueueBase:   QueueBase{sc: sc, config: config},
		redisClient: redisClient,
		cache:       NewCache(redisClient, config),
	}
}

func (p *QueueConfirmation) IsAllowedConnection(c echo.Context) bool {
	// ドメインでqueueが有効ではないので制限されていない
	if _, err := p.cache.Get(c.Request().Context(), enableDomainKey(c)); err == redis.Nil {
		return true
	}

	// 許可済みのコネクション
	allowedID, err := c.Cookie(allowedAccessCookieKey)
	if err != nil && err != http.ErrNoCookie {
		return false
	} else if allowedID != nil {
		_, err := p.cache.Get(c.Request().Context(), allowedID.Value)
		if err == nil {
			return true
		}
	}
	return false
}

func (p *QueueConfirmation) parseWaitingInfoByCookie(c echo.Context) (*WaitingInfo, error) {
	cookie, err := c.Cookie(waitingInfoCookieKey)
	if err != nil {
		if err != http.ErrNoCookie {
			return nil, err
		}
	}
	waitingInfo := WaitingInfo{}
	if cookie != nil {
		if err = p.sc.Decode(waitingInfoCookieKey,
			cookie.Value,
			&waitingInfo); err != nil {
			logrus.Warnf("can't decode waiting info:%s", err)
		}
	}
	return &waitingInfo, nil

}
func (p *QueueConfirmation) getAllowedNo(c echo.Context) (int64, error) {
	// 現在許可されている通り番号
	v, err := p.cache.Get(c.Request().Context(), hostAllowedNumberKey(c))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func (p *QueueConfirmation) disableWaitingInfoCookie(c echo.Context) {
	// Decodeできない場合は無効にする
	c.SetCookie(&http.Cookie{
		Name:     waitingInfoCookieKey,
		MaxAge:   -1,
		Domain:   c.Param("domain"),
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
}
func (p *QueueConfirmation) setAllowedCookie(c echo.Context, waitingInfo *WaitingInfo) error {
	v := p.redisClient.SetEX(c.Request().Context(),
		waitingInfo.ID,
		"1",
		time.Duration(p.config.AllowedAccessSec)*time.Second,
	)
	if v.Err() != nil {
		return v.Err()
	}

	c.SetCookie(&http.Cookie{
		Name:     allowedAccessCookieKey,
		Value:    waitingInfo.ID,
		MaxAge:   p.config.AllowedAccessSec,
		Domain:   c.Param("domain"),
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
	return nil

}

func (p *QueueConfirmation) queueEntry(c echo.Context) error {
	u, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	waitingInfo := &WaitingInfo{
		ID:             u.String(),
		EntryTimestamp: time.Now().Unix(),
	}

	err = p.setWaitingInfoCookie(c, waitingInfo)
	if err != nil {
		return err
	}
	return nil
}

func (p *QueueConfirmation) buildWaitingInfo(c echo.Context, waitingInfo *WaitingInfo) error {
	if waitingInfo.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		waitingInfo.ID = u.String()
		waitingInfo.EntryTimestamp = time.Now().Unix()
	} else if waitingInfo.EntryTimestamp+p.config.EntryDelaySec*int64(time.Second) < time.Now().Unix() {
		v := p.redisClient.Incr(c.Request().Context(), hostSerialNumberKey(c))
		if v.Err() != nil {
			return v.Err()
		}
		waitingInfo.SerialNumber = v.Val()
	}
	return nil

}
func (p *QueueConfirmation) Do(c echo.Context) error {
	if p.IsAllowedConnection(c) {
		return c.String(http.StatusOK, "access allow")
	}

	waitingInfo, err := p.parseWaitingInfoByCookie(c)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, " can't get waiting info")
	}

	// 採番されていない
	if waitingInfo.SerialNumber == 0 {
		if err := p.buildWaitingInfo(c, waitingInfo); err != nil {
			return NewError(http.StatusInternalServerError, err, " can't build waiting info")
		}
	} else {
		allowedNo, err := p.getAllowedNo(c)
		if err != nil {
			return NewError(http.StatusInternalServerError, err, " can't get allowed no")
		}

		// 許可されたとおり番号以上の値を持っている
		if waitingInfo.SerialNumber > 0 && allowedNo > waitingInfo.SerialNumber {
			if err := p.setAllowedCookie(c, waitingInfo); err != nil {
				NewError(http.StatusInternalServerError, err, " can't set allowed key")
			}
			p.disableWaitingInfoCookie(c)
			return c.String(http.StatusOK, "ok")
		}
	}
	err = p.setWaitingInfoCookie(c, waitingInfo)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, "can't save waiting info")
	}

	return c.String(http.StatusTooManyRequests, "please waiting")
}
