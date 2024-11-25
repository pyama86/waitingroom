package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	"github.com/pyama86/waitingroom/waitingroom"
)

func TestQueues_Check(t *testing.T) {
	redisClient := testRedisClient()
	cache := waitingroom.NewCache(redisClient, &waitingroom.Config{})
	type fields struct {
		sc          *securecookie.SecureCookie
		cache       *waitingroom.Cache
		redisClient *redis.Client
		config      *waitingroom.Config
	}
	tests := []struct {
		name              string
		fields            fields
		client            waitingroom.Client
		wantErr           bool
		wantStatus        int
		beforeHook        func(string, *redis.Client)
		expect            func(*testing.T, *waitingroom.Client, *redis.Client)
		expectQueueResult QueueResult
	}{
		{
			name: "now queue and delay take number",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client:     waitingroom.Client{},
			wantErr:    false,
			wantStatus: http.StatusTooManyRequests,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixPermittedNo, 0, 10*time.Second)
			},
			expect: func(t *testing.T, c *waitingroom.Client, r *redis.Client) {
				if c.ID == "" {
					t.Errorf("TestQueuesCheck Client ID is not allow null ID")
				}
				if c.TakeSerialNumberTime != time.Now().Unix()+10 {
					t.Errorf("TestQueuesCheck miss match c.TakeSerialNumberTime")
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
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client: waitingroom.Client{
				ID:                   testRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusTooManyRequests,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixCurrentNo, 31, 10*time.Second)
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixPermittedNo, 1, 10*time.Second)
			},
			expect: func(t *testing.T, c *waitingroom.Client, r *redis.Client) {
				if c.ID == "" {
					t.Errorf("TestQueuesCheck Client ID is not allow null ID")
				}
				if c.SerialNumber == 0 {
					t.Errorf("TestQueuesCheck miss match c.SerialNumber")
				}
			},
			expectQueueResult: QueueResult{
				Enabled:             true,
				PermittedClient:     false,
				SerialNo:            32,
				PermittedNo:         1,
				RemainingWaitSecond: 40,
			},
		},
		{
			name: "queue isn't start",
			fields: fields{
				sc:          secureCookie,
				redisClient: redisClient,
				cache:       cache,
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client:     waitingroom.Client{},
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
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client: waitingroom.Client{
				SerialNumber:         1,
				ID:                   testRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixPermittedNo, 1, 10*time.Second)
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
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client: waitingroom.Client{
				ID:                   testRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixCurrentNo, 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+waitingroom.SuffixPermittedNo, 1, 10*time.Second)
				redisClient.ZAdd(context.Background(), waitingroom.WhiteListKey, &redis.Z{Member: key, Score: 1})
				redisClient.Expire(context.Background(), waitingroom.WhiteListKey, 10*time.Second)
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
			p := &queueHandler{
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
			encoded, err := secureCookie.Encode(waitingroom.ClientCookieKey, tt.client)
			if err != nil {
				panic(err)
			}

			c.Request().AddCookie(&http.Cookie{
				Name:     waitingroom.ClientCookieKey,
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

			if err := p.Check(c); (err != nil) != tt.wantErr {
				t.Errorf("QueueConfirmation.Do() error = %v, wantErr %v", err, tt.wantErr)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("QueueConfirmation.Do() status = %v, want %v", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				parser := &http.Request{Header: http.Header{"Cookie": rec.Header()["Set-Cookie"]}}
				cookie, _ := parser.Cookie(waitingroom.ClientCookieKey)
				got := waitingroom.Client{}
				secureCookie.Decode(waitingroom.ClientCookieKey,
					cookie.Value,
					&got)

				tt.expect(t, &got, redisClient)
			}

			result := QueueResult{}
			err = json.Unmarshal(rec.Body.Bytes(), &result)
			if err != nil {
				t.Errorf("QueueConfirmation.Do() error = %v", err)
			}

			if result.Enabled != tt.expectQueueResult.Enabled {
				t.Errorf("QueueConfirmation.Do() Enabled = %v, want %v", result.Enabled, tt.expectQueueResult.Enabled)
			}
			if result.PermittedClient != tt.expectQueueResult.PermittedClient {
				t.Errorf("QueueConfirmation.Do() PermittedClient = %v, want %v", result.PermittedClient, tt.expectQueueResult.PermittedClient)
			}
			if result.SerialNo != tt.expectQueueResult.SerialNo {
				t.Errorf("QueueConfirmation.Do() SerialNo = %v, want %v", result.SerialNo, tt.expectQueueResult.SerialNo)
			}
			if result.PermittedNo != tt.expectQueueResult.PermittedNo {
				t.Errorf("QueueConfirmation.Do() PermittedNo = %v, want %v", result.PermittedNo, tt.expectQueueResult.PermittedNo)
			}

			if result.RemainingWaitSecond != tt.expectQueueResult.RemainingWaitSecond {
				t.Errorf("QueueConfirmation.Do() RemainingWaitSecond = %v, want %v", result.RemainingWaitSecond, tt.expectQueueResult.RemainingWaitSecond)
			}
		})
	}
}
