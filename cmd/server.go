/*
Copyright © 2023 pyama86

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/pyama86/ngx_waitingroom/waitingroom"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var secureCookie = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32),
)

func init() {
	if os.Getenv("WAITINGROOM_COOKIE_SECRET_HASH_KEY") != "" && os.Getenv("WAITINGROOM_COOKIE_SECRET_BLOCK_KEY") != "" {
		sc := securecookie.New(
			[]byte(os.Getenv("WAITINGROOM_COOKIE_SECRET_HASH_KEY")),
			[]byte(os.Getenv("WAITINGROOM_COOKIE_SECRET_BLOCK_KEY")),
		)
		secureCookie = sc
	}
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "starting waitingroom server",
	Long:  `It is starting waitingroom servercommand.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := &waitingroom.Config{}

		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.AutomaticEnv()
		if err := viper.Unmarshal(&config); err != nil {
			logrus.Fatal(err)
		}

		validate := validator.New()
		if err := validate.Struct(config); err != nil {
			logrus.Fatal(err)
		}
		if err := runServer(config); err != nil {
			logrus.Fatal(err)
		}
	},
}

func runServer(config *waitingroom.Config) error {
	e := echo.New()
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			return c.Request().RequestURI == "/status"
		},
		Format: `{"time":"${time_rfc3339_nano}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}",` +
			`"status":${status},"error":"${error},"latency":"${latency_human}""` + "\n",
	}))

	e.HideBanner = true
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	switch config.LogLevel {
	case "debug":
		e.Logger.SetLevel(log.DEBUG)
	case "info":
		e.Logger.SetLevel(log.INFO)
	case "warn":
		e.Logger.SetLevel(log.WARN)
	case "error":
		e.Logger.SetLevel(log.ERROR)
	}

	e.Logger.Infof("server config: %#v", config)
	redisDB := 0
	if os.Getenv("REDIS_DB") != "" {
		ai, err := strconv.Atoi(os.Getenv("REDIS_DB"))
		if err != nil {
			return err
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
		return err
	}

	e.Use(middleware.Recover())
	cache := waitingroom.NewCache(redisc, config)
	queueConfirmation := waitingroom.NewQueueConfirmation(
		secureCookie,
		config,
		redisc,
		cache,
	)

	e.GET("/status", func(c echo.Context) error {
		var ctx = context.Background()
		_, err := redisc.Ping(ctx).Result()
		if err != nil {
			return waitingroom.NewError(http.StatusInternalServerError, err, "datastore connection error")
		}
		return c.String(http.StatusOK, "ok")
	},
	)
	e.GET("/queues/:domain", queueConfirmation.Do)
	e.GET("/queues/:domain/:enable", queueConfirmation.Do)

	go func() {
		if err := e.Start(config.Listener); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server", err)
		}
	}()

	go func() {
		ac := waitingroom.NewAccessController(
			config,
			redisc,
			cache,
		)
		for {
			if err := ac.Do(ctx, e); err != nil && err != redis.Nil {
				e.Logger.Errorf("error allow worker: %s", err)
			}
			time.Sleep(time.Duration(config.AllowIntervalSec) * time.Second)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	qctx, qcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer qcancel()
	if err := e.Shutdown(qctx); err != nil {
		return err
	}
	return nil
}

func init() {
	serverCmd.PersistentFlags().String("log-level", "info", "log level(debug,info,warn,error)")
	viper.BindPFlag("LogLevel", serverCmd.PersistentFlags().Lookup("log-level"))

	serverCmd.PersistentFlags().String("listener", "localhost:18080", "listen host")
	viper.BindPFlag("Listener", serverCmd.PersistentFlags().Lookup("listener"))

	viper.SetDefault("client_polling_interval_sec", 60)
	viper.SetDefault("allowed_access_sec", 600)
	viper.SetDefault("cache_ttl_sec", 20)
	viper.SetDefault("entry_delay_sec", 10)
	viper.SetDefault("queue_enable_sec", 300)
	viper.SetDefault("allow_interval_sec", 60)
	viper.SetDefault("allow_unit_number", 1000)
	rootCmd.AddCommand(serverCmd)
}
