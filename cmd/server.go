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
	"log/slog"
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
	"github.com/pyama86/ngx_waitingroom/api"
	"github.com/pyama86/ngx_waitingroom/docs"
	"github.com/pyama86/ngx_waitingroom/waitingroom"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
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
		config := waitingroom.Config{}

		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.SetEnvPrefix("WAITINGROOM")
		viper.AutomaticEnv()
		viper.SetConfigType("toml")
		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		} else {
			fmt.Printf("config file read error: %s", err)
		}

		if err := viper.Unmarshal(&config); err != nil {
			log.Fatal(err)
		}

		validate := validator.New(validator.WithRequiredStructEnabled())
		if err := validate.Struct(config); err != nil {
			log.Fatal(err)
		}
		if err := runServer(cmd, &config); err != nil {
			log.Fatal(err)
		}
	},
}

func runServer(cmd *cobra.Command, config *waitingroom.Config) error {
	e := echo.New()

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogMethod:   true,
		LogRemoteIP: true,
		LogError:    true,
		LogHost:     true,
		HandleError: true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				slog.LogAttrs(context.Background(), slog.LevelInfo, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("remote_ip", v.RemoteIP),
					slog.String("host", v.Host),
					slog.String("method", v.Method),
				)
			} else {
				slog.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("remote_ip", v.RemoteIP),
					slog.String("host", v.Host),
					slog.String("method", v.Method),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))

	logLevel := slog.LevelInfo
	switch config.LogLevel {
	case "info":
		logLevel = slog.LevelInfo
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	ops := slog.HandlerOptions{
		Level: logLevel,
	}

	hostname, _ := os.Hostname()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &ops)).With(slog.String("server_host", hostname))
	slog.SetDefault(logger)

	e.HideBanner = true
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if config.EnableOtel {
		otelAgentAddr, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if !ok {
			otelAgentAddr = "127.0.0.1:4317"
		}

		tp, err := initTracer(ctx, otelAgentAddr)
		e.Use(otelecho.Middleware("waitingroom", otelecho.WithTracerProvider(tp)))
		if err != nil {
			return err
		}
		defer func() {
			if err := tp.Shutdown(ctx); err != nil {
				slog.Error(
					fmt.Sprintf("Error shutting down tracer provider: %v", err),
				)
			}
		}()
	}

	slog.Info(fmt.Sprintf("server config: %#v", config))
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
		_, err := redisc.Ping(ctx).Result()
		if err != nil {
			return waitingroom.NewError(http.StatusInternalServerError, err, "datastore connection error")
		}
		return c.String(http.StatusOK, "ok")
	},
	)
	e.GET("/queues/:domain", queueConfirmation.Do)
	e.GET("/queues/:domain/:enable", queueConfirmation.Do)

	v1 := e.Group("/v1")
	api.VironEndpoints(v1)
	api.QueuesEndpoints(v1, redisc, config, cache)
	api.WhiteListEndpoints(v1, redisc)

	docs.SwaggerInfo.Host = config.PublicHost
	dev, err := cmd.PersistentFlags().GetBool("dev")
	if err != nil {
		return waitingroom.NewError(http.StatusInternalServerError, err, "can't parse dev flag")
	}

	if dev {
		docs.SwaggerInfo.Schemes = []string{"http"}
		fmt.Printf("%v", config)
	} else {
		docs.SwaggerInfo.Schemes = []string{"https"}
	}
	middleware.DefaultCORSConfig.AllowHeaders = []string{
		"X-Pagination-Limit",
		"X-Pagination-Total-Pages",
		"X-Pagination-Current-Page",
	}
	v1.GET("/swagger/*", echoSwagger.WrapHandler)
	e.Use(middleware.CORS())
	go func() {
		if err := e.Start(config.Listener); err != nil && err != http.ErrServerClosed {
			log.Fatal("shutting down the server", err)
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
				slog.Error(
					"error permit worker",
					slog.String("error", err.Error()),
				)
			}
			time.Sleep(time.Duration(config.PermitIntervalSec) * time.Second)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	qctx, qcancel := context.WithTimeout(ctx, 10*time.Second)
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
	serverCmd.PersistentFlags().String("public-host", "localhost:18080", "public host for swagger")
	viper.BindPFlag("Listener", serverCmd.PersistentFlags().Lookup("listener"))
	viper.BindPFlag("PublicHost", serverCmd.PersistentFlags().Lookup("public-host"))

	serverCmd.PersistentFlags().Bool("otel", false, "use otel")
	viper.BindPFlag("enable_otel", serverCmd.PersistentFlags().Lookup("otel"))

	serverCmd.PersistentFlags().Bool("dev", false, "dev mode")

	viper.SetDefault("client_polling_interval_sec", 60)
	viper.SetDefault("permitted_access_sec", 600)
	viper.SetDefault("cache_ttl_sec", 20)
	viper.SetDefault("negative_cache_ttl_sec", 10)
	viper.SetDefault("entry_delay_sec", 10)
	viper.SetDefault("queue_enable_sec", 300)
	viper.SetDefault("permit_interval_sec", 60)
	viper.SetDefault("permit_unit_number", 1000)
	viper.SetDefault("public_host", "localhost:18080")
	viper.BindEnv("slack_api_token", "SLACK_API_TOKEN")
	viper.BindEnv("slack_channel", "SLACK_CHANNEL")
	rootCmd.AddCommand(serverCmd)
}

func initTracer(ctx context.Context, otelAgentAddr string) (*sdktrace.TracerProvider, error) {
	client := otlptracehttp.NewClient(
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(otelAgentAddr))

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, err
	}

	resource := resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String("waitingroom"))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
		sdktrace.WithResource(resource),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}
