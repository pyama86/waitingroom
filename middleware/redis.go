package middleware

import (
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

const (
	// RedisKey Redisセッションを保存しているキー
	RedisKey = "Redis"
)

// Redis Redisセッションをコンテキストに保存するミドルウェア
func Redis(redis *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return echo.HandlerFunc(func(c echo.Context) error {
			c.Set(RedisKey, redis)
			return next(c)
		})
	}
}
