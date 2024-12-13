package testutils

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"net/http/httptest"

	"github.com/go-redis/redis/v8"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

func TestRedisClient() *redis.Client {
	redisHost := "127.0.0.1"
	if os.Getenv("REDIS_HOST") != "" {
		redisHost = os.Getenv("REDIS_HOST")
	}
	return redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, 6379),
	})
}

func TestContext(path, method string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	postBody, _ := json.Marshal(params)

	req := httptest.NewRequest(method, path, bytes.NewBuffer(postBody))
	req.Header.Set("Content-Type", "application/json")
	e := echo.New()
	ctx := e.NewContext(req, rec)
	return ctx, rec
}

var SecureCookie = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32),
)

func TestRandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		if err != nil {
			panic(err)
		}
		b[i] = letterRunes[num.Int64()]
	}
	return string(b)
}
