package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
	emiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pyama86/ngx_waitingroom/middleware"
	"github.com/sirupsen/logrus"
)

func healthCheck(c echo.Context) error {
	redisc := c.Get(middleware.RedisKey).(*redis.Client)
	var ctx = context.Background()
	_, err := redisc.Ping(ctx).Result()
	if err != nil {
		return NewError(http.StatusInternalServerError, err, "datastore connection error")
	}
	return c.String(http.StatusOK, "ok")
}

var cookies = map[string]*securecookie.SecureCookie{
	"previous": securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	),
	"current": securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	),
}

func init() {
	if os.Getenv("WAITINGROOM_COOKIE_SECRET_HASH_KEY") != "" && os.Getenv("WAITINGROOM_COOKIE_SECRET_BLOCK_KEY") != "" {
		sc := securecookie.New(
			[]byte(os.Getenv("WAITINGROOM_COOKIE_SECRET_HASH_KEY")),
			[]byte(os.Getenv("WAITINGROOM_COOKIE_SECRET_BLOCK_KEY")),
		)
		cookies["previous"] = sc
		cookies["current"] = sc
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	e := echo.New()
	var ctx = context.Background()
	e.Use(emiddleware.Logger())
	e.Use(emiddleware.Recover())

	redisDB := 0
	if os.Getenv("REDIS_DB") != "" {
		ai, err := strconv.Atoi(os.Getenv("REDIS_DB"))
		if err != nil {
			logrus.Fatal(err)
		}
		redisDB = ai
	}

	redisHost := getEnv("REDIS_HOST", "127.0.0.1")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisOptions := redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
		DB:   redisDB,
	}

	if os.Getenv("REDIS_PASSWORD") != "" {
		redisOptions.Password = os.Getenv("REDIS_PASSWORD")
	}

	redisc := redis.NewClient(&redisOptions)
	_, err := redisc.Ping(ctx).Result()
	if err != nil {
		logrus.Fatal(err)
	}
	e.Use(middleware.Redis(redisc))

	e.GET("/status", healthCheck)
	go func() {
		if err := e.Start(getEnv("WAITINGROOM_LISTEN", "127.0.0.1:8080")); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server", err)
		}

	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logrus.Fatal(err)
	}

}
