package waitingroom

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

func TestQueueConfirmation_enableQueue(t *testing.T) {
	type fields struct {
		QueueBase   QueueBase
		redisClient *redis.Client
	}
	tests := []struct {
		name    string
		key     string
		fields  fields
		c       echo.Context
		wantErr bool
	}{
		{
			name: "ok",
			key:  string(securecookie.GenerateRandomKey(64)),
			fields: fields{
				QueueBase: QueueBase{
					config: &Config{
						QueueEnableSec: 600,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		redisClient := redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
		})

		t.Run(tt.name, func(t *testing.T) {
			p := &QueueConfirmation{
				QueueBase:   tt.fields.QueueBase,
				redisClient: redisClient,
			}
			c, _ := testContext("/", http.MethodPost, map[string]string{"enable": "true"})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(tt.key)

			if err := p.enableQueue(c); (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.enableQueue() error = %v, wantErr %v", err, tt.wantErr)
			}

			rv := redisClient.Get(c.Request().Context(), tt.key+"_enable")
			if rv.Err() != nil {
				t.Errorf("got error %v", rv.Err())
			}
		})
	}
}

func BenchmarkQueueEnable_Do(b *testing.B) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
	})
	p := &QueueConfirmation{
		QueueBase: QueueBase{
			config: &Config{
				QueueEnableSec: 600,
			},
		},
		redisClient: redisClient,
	}

	for i := 0; i < b.N; i++ {
		c, _ := testContext("/", http.MethodPost, map[string]string{"enable": "true"})
		c.SetPath("/queues/:domain")
		c.SetParamNames("domain")
		c.SetParamValues(fmt.Sprintf("%d", i))
		if err := p.enableQueue(c); err != nil {
			b.Errorf("QueueEnable.Do() error = %v", err)
		}
	}
}

func TestQueueConfirmation_isAllowedConnection(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		QueueBase   QueueBase
		beforeHook  func(string, *redis.Client)
		waitingInfo *WaitingInfo
		want        bool
	}{
		{
			name: "disabled",
			key:  string(securecookie.GenerateRandomKey(64)),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), key)
			},
			waitingInfo: &WaitingInfo{
				SerialNumber: 100,
			},
			want: true,
		},
		{
			name: "keep waiting",
			key:  string(securecookie.GenerateRandomKey(64)),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_enable", "", 5*time.Second)
				redisClient.Del(context.Background(), key)
			},
			waitingInfo: &WaitingInfo{
				SerialNumber: 100,
			},
			want: false,
		},
		{
			name: "allowed",
			key:  string(securecookie.GenerateRandomKey(64)),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_enable", "", 5*time.Second)
				redisClient.SetEX(context.Background(), key, "", 5*time.Second)
			},
			waitingInfo: &WaitingInfo{
				SerialNumber: 100,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := redis.NewClient(&redis.Options{
				Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
			})

			p := &QueueConfirmation{
				QueueBase:   tt.QueueBase,
				cache:       NewCache(redisClient, &Config{}),
				redisClient: redisClient,
			}
			c, _ := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(tt.key)
			tt.beforeHook(tt.key, redisClient)

			tt.waitingInfo.ID = tt.key
			if got := p.isAllowedConnection(c, tt.waitingInfo); got != tt.want {
				t.Errorf("QueueConfirmation.isAllowedConnection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueueConfirmation_parseWaitingInfoByCookie(t *testing.T) {
	tests := []struct {
		name             string
		c                echo.Context
		getEncodedCookie func(*securecookie.SecureCookie, *WaitingInfo) string
		want             *WaitingInfo
		wantErr          bool
	}{
		{
			name: "ok",
			want: &WaitingInfo{
				ID:             "1",
				EntryTimestamp: 2,
				SerialNumber:   3,
			},
			getEncodedCookie: func(sc *securecookie.SecureCookie, w *WaitingInfo) string {
				encoded, _ := sc.Encode(waitingInfoCookieKey, w)
				return encoded
			},
		},
		{
			name: "broken cookie",
			getEncodedCookie: func(sc *securecookie.SecureCookie, w *WaitingInfo) string {
				t := struct {
					dummy string
				}{
					dummy: "hote",
				}
				encoded, _ := sc.Encode("hoge", t)
				return encoded
			},
			wantErr: false,
		},
		{
			name:    "not present cookie",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := redis.NewClient(&redis.Options{
				Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
			})

			secureCookie := securecookie.New(
				securecookie.GenerateRandomKey(64),
				securecookie.GenerateRandomKey(32),
			)

			p := &QueueConfirmation{
				QueueBase:   QueueBase{sc: secureCookie},
				cache:       NewCache(redisClient, &Config{}),
				redisClient: redisClient,
			}

			c, _ := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues("example.com")

			if tt.want != nil {
				encoded := tt.getEncodedCookie(secureCookie, tt.want)
				c.Request().AddCookie(&http.Cookie{
					Name:     waitingInfoCookieKey,
					Value:    encoded,
					MaxAge:   100,
					Domain:   "example.com",
					Path:     "/",
					Secure:   true,
					HttpOnly: true,
				})
			}
			got, err := p.parseWaitingInfoByCookie(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.parseWaitingInfoByCookie() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want == nil {
				tt.want = &WaitingInfo{}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QueueConfirmation.parseWaitingInfoByCookie() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueueConfirmation_getAllowedNo(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		beforeHook func(string, *redis.Client)
		want       int64
		wantErr    bool
	}{
		{
			name:    "not_set",
			key:     string(securecookie.GenerateRandomKey(64)),
			wantErr: true,
		},
		{
			name: "ok",
			key:  string(securecookie.GenerateRandomKey(64)),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_allowed_no", 10, 10*time.Second)
			},
			want:    10,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := redis.NewClient(&redis.Options{
				Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
			})

			p := &QueueConfirmation{
				cache:       NewCache(redisClient, &Config{}),
				redisClient: redisClient,
			}

			c, _ := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(tt.key)

			if tt.beforeHook != nil {
				tt.beforeHook(tt.key, redisClient)
			}
			got, err := p.getAllowedNo(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.getAllowedNo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("QueueConfirmation.getAllowedNo() = %v, want %v", got, tt.want)
			}
		})
	}
}
