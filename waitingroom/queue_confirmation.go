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

func (p *QueueConfirmation) isAllowedConnection(c echo.Context, waitingInfo *WaitingInfo) bool {
	// ドメインでqueueが有効ではないので制限されていない
	if _, err := p.cache.Get(c.Request().Context(), p.enableDomainKey(c)); err == redis.Nil {
		return true
	}

	// 許可済みのコネクション
	if waitingInfo.ID != "" && waitingInfo.SerialNumber != 0 {
		_, err := p.cache.Get(c.Request().Context(), waitingInfo.ID)
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
	v, err := p.cache.Get(c.Request().Context(), p.hostAllowedNumberKey(c))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func (p *QueueConfirmation) allowAccess(c echo.Context, waitingInfo *WaitingInfo) error {
	v := p.redisClient.SetEX(c.Request().Context(),
		waitingInfo.ID,
		strconv.FormatInt(waitingInfo.SerialNumber, 10),
		time.Duration(p.config.AllowedAccessSec)*time.Second,
	)
	return v.Err()
}

func (p *QueueConfirmation) buildWaitingInfo(c echo.Context, waitingInfo *WaitingInfo) error {
	if waitingInfo.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		waitingInfo.ID = u.String()
		waitingInfo.EntryTimestamp = time.Now().Unix()
	} else if waitingInfo.EntryTimestamp != 0 &&
		waitingInfo.EntryTimestamp+p.config.EntryDelaySec*int64(time.Second) < time.Now().Unix() {
		v := p.redisClient.Incr(c.Request().Context(), p.hostSerialNumberKey(c))
		if v.Err() != nil {
			return v.Err()
		}
		waitingInfo.SerialNumber = v.Val()
	}
	return nil
}

func (p *QueueConfirmation) enableQueue(c echo.Context) error {
	// ドメインをキューの対象にする
	if c.FormValue("enable") != "" {
		// 有効になっているドメインの個別キー
		r := p.redisClient.Set(c.Request().Context(),
			p.enableDomainKey(c), "1",
			time.Duration(p.config.QueueEnableSec)*time.Second)
		if err := r.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (p *QueueConfirmation) Do(c echo.Context) error {
	waitingInfo, err := p.parseWaitingInfoByCookie(c)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, " can't build waiting info")
	}

	if p.isAllowedConnection(c, waitingInfo) {
		return c.String(http.StatusOK, "ok")
	}

	// キューの有効時間更新
	if err := p.enableQueue(c); err != nil {
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
		if allowedNo > waitingInfo.SerialNumber {
			if err := p.allowAccess(c, waitingInfo); err != nil {
				NewError(http.StatusInternalServerError, err, " can't set allowed key")
			}
			return c.String(http.StatusOK, "ok")
		}
	}
	err = p.setWaitingInfoCookie(c, waitingInfo)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, "can't save waiting info")
	}

	return c.String(http.StatusTooManyRequests, "please waiting")
}
