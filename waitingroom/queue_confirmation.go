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
	if _, err := p.cache.Get(c.Request().Context(), p.allowNoKey(c)); err == redis.Nil {
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
	v, err := p.cache.Get(c.Request().Context(), p.allowNoKey(c))
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

// エントリー時間から指定秒数経過していれば採番する
// 指定秒数待つのは多重にリクエストされた場合を想定して、クッキーがひとつに収束するのを待つ
func (p *QueueConfirmation) takeNumberIfPossible(c echo.Context, waitingInfo *WaitingInfo) error {
	if waitingInfo.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		waitingInfo.ID = u.String()
		waitingInfo.EntryTimestamp = time.Now().Unix()
	} else if waitingInfo.EntryTimestamp != 0 &&
		waitingInfo.EntryTimestamp+p.config.EntryDelaySec < time.Now().Unix() {
		v := p.redisClient.Incr(c.Request().Context(), p.hostCurrentNumberKey(c))
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
		pipe := p.redisClient.Pipeline()
		pipe.SetNX(c.Request().Context(), p.allowNoKey(c), "1", 0)
		pipe.Expire(c.Request().Context(),
			p.allowNoKey(c), time.Duration(p.config.QueueEnableSec)*time.Second)
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

	if p.isAllowedConnection(c, waitingInfo) {
		return c.JSON(http.StatusOK, "")
	}

	// キューの有効時間更新
	if err := p.enableQueue(c); err != nil {
		return NewError(http.StatusInternalServerError, err, " can't get waiting info")
	}

	var allowedNo int64
	var serialNo int
	// 採番されていない
	if waitingInfo.SerialNumber == 0 {
		if err := p.takeNumberIfPossible(c, waitingInfo); err != nil {
			return NewError(http.StatusInternalServerError, err, " can't build waiting info")
		}
	} else {
		an, err := p.getAllowedNo(c)
		if err != nil {
			return NewError(http.StatusInternalServerError, err, " can't get allowed no")
		}

		allowedNo = an
		// 許可されたとおり番号以上の値を持っている
		if allowedNo > waitingInfo.SerialNumber {
			if err := p.allowAccess(c, waitingInfo); err != nil {
				NewError(http.StatusInternalServerError, err, " can't set allowed key")
			}
			return c.JSON(http.StatusOK, "")
		}
		serialNo = int(waitingInfo.SerialNumber)
	}
	err = p.setWaitingInfoCookie(c, waitingInfo)
	if err != nil {
		return NewError(http.StatusInternalServerError, err, "can't save waiting info")
	}

	return c.String(http.StatusTooManyRequests, fmt.Sprintf(`{"serial_no": %d, "allowed_no": %d }`, serialNo, allowedNo))
}
