package waitingroom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http/httptest"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetOutput(io.Discard)
}
func testContext(path, method string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	postBody, _ := json.Marshal(params)

	req := httptest.NewRequest(method, path, bytes.NewBuffer(postBody))
	req.Header.Set("Content-Type", "application/json")
	e := echo.New()
	ctx := e.NewContext(req, rec)
	return ctx, rec
}

func testRedisClient() *redis.Client {
	redisHost := "127.0.0.1"
	if os.Getenv("REDIS_HOST") != "" {
		redisHost = os.Getenv("REDIS_HOST")
	}
	return redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, 6379),
	})
}

func testRandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
