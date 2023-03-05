package waitingroom

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

func TestAccessController_Do(t *testing.T) {
	tests := []struct {
		ctx        context.Context
		beforeHook func(string, *redis.Client)
		name       string
		domain     string
		want       int
		wantErr    bool
	}{
		{
			name:   "ok",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.ZAdd(context.Background(), enableDomainKey, &redis.Z{
					Member: key,
					Score:  1,
				})
				redisClient.SetEX(context.Background(), key+suffixPermittedNo, "1", 3*time.Second)
			},
			want: 1001,
		},
		{
			name:   "skip update because not target domain",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.ZAdd(context.Background(), enableDomainKey, &redis.Z{
					Member: "hoge",
					Score:  1,
				})
			},
			want: 0,
		},
		{
			name:   "skip update because not enabled domain",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.ZAdd(context.Background(), enableDomainKey, &redis.Z{
					Member: "hoge",
					Score:  1,
				})
			},
			want: 0,
		},
		{
			name:   "skip update because doesn't reach update time",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.ZAdd(context.Background(), enableDomainKey, &redis.Z{
					Member: "hoge",
					Score:  1,
				})
				redisClient.SetEX(context.Background(), key+suffixPermittedNo, "1", 600*time.Second)
				redisClient.SetEX(context.Background(), key+suffixPermittedNoLock, "1", 10*time.Second)
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			a := &AccessController{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
				redisClient: redisClient,
				cache:       NewCache(redisClient, &Config{}),
			}

			if tt.beforeHook != nil {
				tt.beforeHook(tt.domain, redisClient)
			}
			e := echo.New()
			if err := a.Do(context.Background(), e); (err != nil) != tt.wantErr {
				t.Errorf("AccessController.Do() error = %v, wantErr %v", err, tt.wantErr)
			}

			v, err := redisClient.Get(context.Background(), tt.domain+suffixPermittedNo).Result()
			if tt.want != 0 {
				if (err != nil) != tt.wantErr {
					t.Errorf("AccessController.Do() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if v != strconv.Itoa(int(tt.want)) {
					t.Errorf("AccessController.Do() = %v, want %v", v, tt.want)
				}

			} else {
				if err != redis.Nil {
					t.Errorf("AccessController.Do() value = %v error = %v", v, err)
					return
				}
			}
		})
	}
}
