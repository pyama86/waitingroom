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
		QueueBase QueueBase
	}
	tests := []struct {
		name       string
		key        string
		want       string
		fields     fields
		c          echo.Context
		beforeHook func(string, *redis.Client)
		wantErr    bool
	}{
		{
			name: "ok",
			key:  testRandomString(20),
			fields: fields{
				QueueBase: QueueBase{
					config: &Config{
						QueueEnableSec: 600,
					},
				},
			},
			want:    "0",
			wantErr: false,
		},
		{
			name: "don't overrite allow_no",
			key:  testRandomString(20),
			fields: fields{
				QueueBase: QueueBase{
					config: &Config{
						QueueEnableSec: 600,
					},
				},
			},
			want: "2",
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Set(context.Background(), key+"_allow_no", "2", 0)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		redisClient := testRedisClient()

		t.Run(tt.name, func(t *testing.T) {
			tt.fields.QueueBase.cache = NewCache(redisClient, &Config{})
			tt.fields.QueueBase.redisClient = redisClient
			p := &QueueConfirmation{
				QueueBase: tt.fields.QueueBase,
			}
			c, _ := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain/:enable")
			c.SetParamNames("domain", "enable")
			c.SetParamValues(tt.key, "true")

			if tt.beforeHook != nil {
				tt.beforeHook(tt.key, redisClient)
			}

			if err := p.enableQueue(c); (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.enableQueue() error = %v, wantErr %v", err, tt.wantErr)
			}

			rv := redisClient.Get(c.Request().Context(), tt.key+"_allow_no")
			if rv.Err() != nil {
				t.Errorf("got error %v", rv.Err())
			}

			if rv.Val() != tt.want {
				t.Errorf("miss match value got:%v want:%v", rv.Val(), tt.want)
			}

			ev := redisClient.TTL(c.Request().Context(), tt.key+"_allow_no")
			if ev.Err() != nil {
				t.Errorf("got error %v", ev.Err())
			}
			if ev.Val() != 600*time.Second {
				t.Errorf("got ttl %v", ev.Val())
			}

			sv := redisClient.SIsMember(c.Request().Context(), enableDomainKey, tt.key)
			if !sv.Val() {
				t.Errorf("%v is not enabled", tt.key)
			}
		})
	}
}

func BenchmarkQueueEnable_Do(b *testing.B) {
	redisClient := testRedisClient()
	p := &QueueConfirmation{
		QueueBase: QueueBase{
			redisClient: redisClient,
			config: &Config{
				QueueEnableSec: 600,
			},
		},
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
			key:  testRandomString(20),
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
			key:  testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_allow_no", "", 5*time.Second)
				redisClient.Del(context.Background(), key)
			},
			waitingInfo: &WaitingInfo{
				SerialNumber: 100,
			},
			want: false,
		},
		{
			name: "allowed",
			key:  testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_allow_no", "", 5*time.Second)
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
			redisClient := testRedisClient()
			tt.QueueBase.cache = NewCache(redisClient, &Config{})
			tt.QueueBase.redisClient = redisClient
			p := &QueueConfirmation{
				QueueBase: tt.QueueBase,
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
				ID:           "1",
				SerialNumber: 3,
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
			redisClient := testRedisClient()
			secureCookie := securecookie.New(
				securecookie.GenerateRandomKey(64),
				securecookie.GenerateRandomKey(32),
			)

			p := &QueueConfirmation{
				QueueBase: QueueBase{
					sc:          secureCookie,
					cache:       NewCache(redisClient, &Config{}),
					redisClient: redisClient,
				},
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
			key:     testRandomString(20),
			wantErr: true,
		},
		{
			name: "ok",
			key:  testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_allow_no", 10, 10*time.Second)
			},
			want:    10,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			p := &QueueConfirmation{
				QueueBase: QueueBase{
					cache:       NewCache(redisClient, &Config{}),
					redisClient: redisClient,
				},
			}

			c, _ := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(tt.key)

			if tt.beforeHook != nil {
				tt.beforeHook(tt.key, redisClient)
			}
			got, err := p.getAllowedNo(c.Request().Context(), tt.key, true)
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

func TestQueueConfirmation_allowAccess(t *testing.T) {
	type fields struct {
		QueueBase QueueBase
	}
	tests := []struct {
		name        string
		key         string
		fields      fields
		waitingInfo *WaitingInfo
		wantErr     bool
	}{
		{
			name: "ok",
			waitingInfo: &WaitingInfo{
				ID: testRandomString(20),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			p := &QueueConfirmation{
				QueueBase: QueueBase{
					config: &Config{
						AllowedAccessSec: 10,
					},
					cache:       NewCache(redisClient, &Config{}),
					redisClient: redisClient,
				},
			}

			c, _ := testContext("/", http.MethodPost, map[string]string{})
			if err := p.allowAccess(c, tt.waitingInfo); (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.allowAccess() error = %v, wantErr %v", err, tt.wantErr)
			}

			ev := redisClient.TTL(c.Request().Context(), tt.waitingInfo.ID)
			if ev.Err() != nil {
				t.Errorf("got error %v", ev.Err())
			}
			if ev.Val() != 10*time.Second {
				t.Errorf("got ttl %v", ev.Val())
			}

		})
	}
}

func TestQueueConfirmation_takeNumberIfPossible(t *testing.T) {
	tests := []struct {
		name             string
		waitingInfo      *WaitingInfo
		wantSerialNumber int64
		wantErr          bool
	}{
		{
			name:        "nothing ID",
			waitingInfo: &WaitingInfo{},
			wantErr:     false,
		},
		{
			name:             "entry now",
			waitingInfo:      &WaitingInfo{},
			wantSerialNumber: 0,
			wantErr:          false,
		},
		{
			name: "entry before 11sec",
			waitingInfo: &WaitingInfo{
				ID:                   testRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 11,
			},
			wantSerialNumber: 1,
			wantErr:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			p := &QueueConfirmation{
				QueueBase: QueueBase{
					config: &Config{
						EntryDelaySec: 10,
					},
					cache:       NewCache(redisClient, &Config{}),
					redisClient: redisClient,
				},
			}

			c, _ := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(testRandomString(20))

			if err := p.takeNumberIfPossible(c, tt.waitingInfo); (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.takeNumberIfPossible() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.waitingInfo.ID == "" {
				t.Error("QueueConfirmation.takeNumberIfPossible() ID is empty")
			}

			if tt.waitingInfo.SerialNumber != tt.wantSerialNumber {
				t.Errorf("QueueConfirmation.takeNumberIfPossible() got seriarl number %d want %d", tt.waitingInfo.SerialNumber, tt.wantSerialNumber)
			}
		})
	}
}
