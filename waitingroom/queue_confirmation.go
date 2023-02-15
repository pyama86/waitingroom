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
) *QueueConfirmation {
	return &QueueConfirmation{
		QueueBase: QueueBase{
			sc:          sc,
			config:      config,
			redisClient: redisClient,
			cache:       NewCache(redisClient, config),
		},
	}
}

func (p *QueueConfirmation) isAllowedConnection(c echo.Context, waitingInfo *WaitingInfo) bool {
	// ドメインでqueueが有効ではないので制限されていない
	if _, err := p.cache.Get(c.Request().Context(), p.allowNoKey(c.Param(paramDomainKey))); err == redis.Nil {
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
		}
	}
	return &waitingInfo, nil

}

func (p *QueueConfirmation) allowAccess(c echo.Context, waitingInfo *WaitingInfo) error {
	_, err := p.redisClient.SetEX(c.Request().Context(),
		waitingInfo.ID,
		strconv.FormatInt(waitingInfo.SerialNumber, 10),
		time.Duration(p.config.AllowedAccessSec)*time.Second,
	).Result()
	return err
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
		ok, err := p.redisClient.SetNX(c.Request().Context(), p.hostDelayTakeNumberKey(c), "1", 0).Result()
		if err != nil {
			return err
		}

		if ok {
			_, err := p.redisClient.Expire(c.Request().Context(),
				p.hostDelayTakeNumberKey(c), time.Duration(p.config.EntryDelaySec)*time.Second).Result()
			if err != nil {
				return err
			}
		}
	} else {
		if _, err := p.cache.Get(c.Request().Context(), p.hostDelayTakeNumberKey(c)); err == redis.Nil {
			v, err := p.redisClient.Incr(c.Request().Context(), p.hostCurrentNumberKey(c)).Result()
			if err != nil {
				return err
			}
			waitingInfo.SerialNumber = v
		}
	}
	return nil
}

func (p *QueueConfirmation) enableQueue(c echo.Context) error {
	if c.Param("enable") != "" {
		pipe := p.redisClient.Pipeline()
		pipe.SetNX(c.Request().Context(), p.allowNoKey(c.Param(paramDomainKey)), "1", 0)
		pipe.Expire(c.Request().Context(),
			p.allowNoKey(c.Param(paramDomainKey)), time.Duration(p.config.QueueEnableSec)*time.Second)
		pipe.SAdd(c.Request().Context(), enableDomainKey, c.Param(paramDomainKey))
		_, err := pipe.Exec(c.Request().Context())
		return err
	}
	return nil
}

func (p *QueueConfirmation) Do(c echo.Context) error {
	waitingInfo, err := p.parseWaitingInfoByCookie(c)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, " can't build waiting info")
	}

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
		an, err := p.getAllowedNo(c.Request().Context(), c.Param(paramDomainKey))
		if err != nil {
			return NewError(http.StatusInternalServerError, err, " can't get allowed no")
		}

		allowedNo = an
		// 許可されたとおり番号以上の値を持っている
		if allowedNo > waitingInfo.SerialNumber {
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

	return c.String(http.StatusTooManyRequests, fmt.Sprintf(`{"serial_no": %d, "allowed_no": %d }`, serialNo, allowedNo))
}
