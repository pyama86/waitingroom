package waitingroom

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
)

func TestQueueConfirmation_Do(t *testing.T) {
	redisClient := testRedisClient()
	cache := NewCache(redisClient, &Config{})
	type fields struct {
		sc          *securecookie.SecureCookie
		cache       *Cache
		redisClient *redis.Client
		config      *Config
	}
	tests := []struct {
		name              string
		fields            fields
		client            Client
		wantErr           bool
		wantStatus        int
		beforeHook        func(string, *redis.Client)
		expect            func(*testing.T, *Client, *redis.Client)
		expectQueueResult QueueResult
	}{
		{
			name: "now queue and delay take number",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
				},
			},
			client:     Client{},
			wantErr:    false,
			wantStatus: http.StatusTooManyRequests,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+SuffixPermittedNo, 1, 10*time.Second)
			},
			expect: func(t *testing.T, c *Client, r *redis.Client) {
				if c.ID == "" {
					t.Errorf("TestQueueConfirmation_Do Client ID is not allow null ID")
				}
				if c.TakeSerialNumberTime != time.Now().Unix()+10 {
					t.Errorf("TestQueueConfirmation_Do miss match c.TakeSerialNumberTime")
				}
			},
			expectQueueResult: QueueResult{
				Enabled:         true,
				PermittedClient: false,
				SerialNo:        0,
				PermittedNo:     0,
			},
		},
		{
			name: "now queue and take number",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
				},
			},
			client: Client{
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusTooManyRequests,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+SuffixPermittedNo, 1, 10*time.Second)
			},
			expect: func(t *testing.T, c *Client, r *redis.Client) {
				if c.ID == "" {
					t.Errorf("TestQueueConfirmation_Do Client ID is not allow null ID")
				}
				if c.SerialNumber == 0 {
					t.Errorf("TestQueueConfirmation_Do miss match c.SerialNumber")
				}
			},
			expectQueueResult: QueueResult{
				Enabled:         true,
				PermittedClient: false,
				SerialNo:        2,
				PermittedNo:     1,
			},
		},
		{
			name: "queue isn't start",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
				},
			},
			client:     Client{},
			wantErr:    false,
			wantStatus: http.StatusOK,
			expectQueueResult: QueueResult{
				Enabled:         false,
				PermittedClient: false,
				SerialNo:        0,
				PermittedNo:     0,
			},
		},
		{
			name: "permit access",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
				},
			},
			client: Client{
				SerialNumber:         1,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+SuffixPermittedNo, 1, 10*time.Second)
			},
			expectQueueResult: QueueResult{
				Enabled:         true,
				PermittedClient: true,
				SerialNo:        0,
				PermittedNo:     0,
			},
		},
		{
			name: "is in whitelist",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
				},
			},
			client: Client{
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+SuffixPermittedNo, 1, 10*time.Second)
				redisClient.ZAdd(context.Background(), WhiteListKey, &redis.Z{Member: key, Score: 1})
			},
			expectQueueResult: QueueResult{
				Enabled:         false,
				PermittedClient: false,
				SerialNo:        0,
				PermittedNo:     0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &QueueConfirmation{
				sc:          tt.fields.sc,
				cache:       tt.fields.cache,
				redisClient: tt.fields.redisClient,
				config:      tt.fields.config,
			}

			domain := testRandomString(20)
			c, rec := testContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(domain)
			defer rec.Result().Body.Close()
			encoded, err := secureCookie.Encode(clientCookieKey, tt.client)
			if err != nil {
				panic(err)
			}

			c.Request().AddCookie(&http.Cookie{
				Name:     clientCookieKey,
				Value:    encoded,
				MaxAge:   10,
				Domain:   domain,
				Path:     "/",
				Secure:   true,
				HttpOnly: true,
			})

			if tt.beforeHook != nil {
				tt.beforeHook(domain, redisClient)
			}

			if err := p.Do(c); (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.Do() error = %v, wantErr %v", err, tt.wantErr)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("QueueConfirmation.Do() status = %v, want %v", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				parser := &http.Request{Header: http.Header{"Cookie": rec.Header()["Set-Cookie"]}}
				cookie, _ := parser.Cookie(clientCookieKey)
				got := Client{}
				secureCookie.Decode(clientCookieKey,
					cookie.Value,
					&got)

				tt.expect(t, &got, redisClient)
			}

			result := QueueResult{}
			err = json.Unmarshal(rec.Body.Bytes(), &result)
			if err != nil {
				t.Errorf("QueueConfirmation.Do() error = %v", err)
			}

			if !reflect.DeepEqual(result, tt.expectQueueResult) {
				t.Errorf("QueueConfirmation.Do() result = %v, expect %v", result, tt.expectQueueResult)
			}
		})
	}
}
