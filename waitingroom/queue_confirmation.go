package waitingroom

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

type QueueConfirmation struct {
	QueueBase
}

func NewQueueConfirmation(
	sc *securecookie.SecureCookie,
	config *Config,
	redisClient *redis.Client,
	cache *Cache,
) *QueueConfirmation {
	return &QueueConfirmation{
		QueueBase: QueueBase{
			sc:          sc,
			config:      config,
			redisClient: redisClient,
			cache:       cache,
		},
	}
}

func (p *QueueConfirmation) isAllowedConnection(c echo.Context, waitingInfo *WaitingInfo) bool {
	// ドメインでqueueが有効ではないので制限されていない
	if _, err := p.cache.GetAndFetchIfExpired(c.Request().Context(), p.allowNoKey(c.Param(paramDomainKey))); err == redis.Nil {
		return true
	}

	// 許可済みのコネクション
	if waitingInfo.ID != "" && waitingInfo.SerialNumber != 0 {
		_, err := p.cache.GetAndFetchIfExpired(c.Request().Context(), waitingInfo.ID)
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
			c.Logger().Warnf("can't decode cookie: %s", err)
			c.SetCookie(&http.Cookie{
				Name:     waitingInfoCookieKey,
				MaxAge:   -1,
				Domain:   c.Param(paramDomainKey),
				Path:     "/",
				Secure:   true,
				HttpOnly: true,
			})
			return nil, err
		}
	}
	return &waitingInfo, nil

}

func (p *QueueConfirmation) allowAccess(c echo.Context, waitingInfo *WaitingInfo) error {
	return p.redisClient.SetEX(c.Request().Context(),
		waitingInfo.ID,
		strconv.FormatInt(waitingInfo.SerialNumber, 10),
		time.Duration(p.config.AllowedAccessSec)*time.Second,
	).Err()
}

// エントリー時間から指定秒数経過していれば採番する
// 指定秒数待つのは多重にリクエストされた場合を想定して、クッキーがひとつに収束するのを待つ
func (p *QueueConfirmation) takeNumberIfPossible(c echo.Context, waitingInfo *WaitingInfo) error {
	if waitingInfo.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		waitingInfo.ID = u.String()
		waitingInfo.TakeSerialNumberTime = time.Now().Unix() + p.config.EntryDelaySec
	} else if waitingInfo.TakeSerialNumberTime > 0 && waitingInfo.TakeSerialNumberTime < time.Now().Unix() {
		pipe := p.redisClient.Pipeline()
		incr := pipe.Incr(c.Request().Context(), p.hostCurrentNumberKey(c.Param(paramDomainKey)))
		pipe.Expire(c.Request().Context(),
			p.hostCurrentNumberKey(c.Param(paramDomainKey)), time.Duration(p.config.QueueEnableSec)*time.Second).Result()
		if _, err := pipe.Exec(c.Request().Context()); err != nil {
			return err
		}

		waitingInfo.SerialNumber = incr.Val()
	}
	return nil
}

func (p *QueueConfirmation) enableQueue(c echo.Context) error {
	cacheKey := p.allowNoKey(c.Param(paramDomainKey)) + "_cache"

	if c.Param("enable") != "" && !p.cache.Exists(cacheKey) {
		pipe := p.redisClient.Pipeline()
		// 値があれば上書きしない、なければ作る
		pipe.SetNX(c.Request().Context(), p.allowNoKey(c.Param(paramDomainKey)), "0", 0)
		pipe.Expire(c.Request().Context(),
			p.allowNoKey(c.Param(paramDomainKey)), time.Duration(p.config.QueueEnableSec)*time.Second)
		pipe.SAdd(c.Request().Context(), enableDomainKey, c.Param(paramDomainKey))
		_, err := pipe.Exec(c.Request().Context())
		if err != nil {
			return err
		}
		p.cache.Set(cacheKey, "1", time.Duration(p.config.QueueEnableSec/2)*time.Second)
	}
	return nil
}

func (p *QueueConfirmation) Do(c echo.Context) error {
	waitingInfo, err := p.parseWaitingInfoByCookie(c)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, " can't build waiting info")
	}

	c.Logger().Debugf("domain %s request waiting info: %#v", c.Param(paramDomainKey), waitingInfo)
	// キューの有効時間更新
	if err := p.enableQueue(c); err != nil {
		return NewError(http.StatusInternalServerError, err, " can't get waiting info")
	}

	if p.isAllowedConnection(c, waitingInfo) {
		return c.JSON(http.StatusOK, "allowed connection")
	}

	var allowedNo int64
	var serialNo int
	// 採番されていない
	if waitingInfo.SerialNumber == 0 {
		if err := p.takeNumberIfPossible(c, waitingInfo); err != nil {
			return NewError(http.StatusInternalServerError, err, " can't build waiting info")
		}
	} else {
		an, err := p.getAllowedNo(c.Request().Context(), c.Param(paramDomainKey), true)
		if err != nil {
			return NewError(http.StatusInternalServerError, err, " can't get allowed no")
		}

		allowedNo = an
		c.Logger().Debugf("allowed no: %d serial_number: %d", allowedNo, waitingInfo.SerialNumber)

		// 許可されたとおり番号以上の値を持っている
		if allowedNo >= waitingInfo.SerialNumber {
			if err := p.allowAccess(c, waitingInfo); err != nil {
				NewError(http.StatusInternalServerError, err, " can't set allowed key")
			}
			return c.JSON(http.StatusOK, "allow connection")
		}
		serialNo = int(waitingInfo.SerialNumber)
	}
	err = p.setWaitingInfoCookie(c, waitingInfo)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, "can't save waiting info")
	}

	c.Logger().Debugf("domain %s response waiting info: %#v", c.Param(paramDomainKey), waitingInfo)
	return c.String(http.StatusTooManyRequests, fmt.Sprintf(`{"serial_no": %d, "allowed_no": %d }`, serialNo, allowedNo))
}
