package waitingroom

import (
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

type QueueEnable struct {
	config      *Config
	redisClient *redis.Client
}

func NewQueueEnable(
	config *Config,
	redisClient *redis.Client,
) *QueueEnable {
	return &QueueEnable{
		config:      config,
		redisClient: redisClient,
	}
}

// キューの開始
// キューはTTLが切れたら自動で終了するので
// キューを継続する場合は継続的にこのエンドポイントが叩かれる必要がある
// 通常はnginxでrate limitに該当したエンドポイントで叩かれることを想定している
func (p *QueueEnable) Do(c echo.Context) error {
	pipe := p.redisClient.Pipeline()
	// 有効になっているドメインの個別キー
	pipe.Set(c.Request().Context(), enableDomainKey(c), "1", time.Duration(p.config.QueueEnableSec)*time.Second)

	// 有効になっているドメインのリスト
	pipe.SAdd(c.Request().Context(), enableDomainsKey(), c.Param("domain"))
	pipe.Expire(c.Request().Context(), enableDomainsKey(), time.Duration(p.config.QueueEnableSec*10)*time.Second)
	if _, err := pipe.Exec(c.Request().Context()); err != nil {
		return NewError(http.StatusInternalServerError, err, "can't set enable flag")
	}

	return c.String(http.StatusCreated, "ok")
}
