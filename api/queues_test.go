package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	waitingroom "github.com/pyama86/waitingroom/domain"
	"github.com/pyama86/waitingroom/repository"
	"github.com/pyama86/waitingroom/testutils"
)

func TestQueues_Check(t *testing.T) {
	redisClient := testutils.TestRedisClient()
	type fields struct {
		sc          *securecookie.SecureCookie
		config      *waitingroom.Config
		redisClient *redis.Client
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
				sc:          testutils.SecureCookie,
				redisClient: redisClient,
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
				redisClient.SetEX(context.Background(), key+"_current_no", 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+"_permitted_no", 0, 10*time.Second)
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
				sc:          testutils.SecureCookie,
				redisClient: redisClient,
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client: waitingroom.Client{
				ID:                   testutils.TestRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusTooManyRequests,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_current_no", 31, 10*time.Second)
				redisClient.SetEX(context.Background(), key+"_permitted_no", 1, 10*time.Second)
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
				sc:          testutils.SecureCookie,
				redisClient: redisClient,
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
				sc:          testutils.SecureCookie,
				redisClient: redisClient,
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client: waitingroom.Client{
				SerialNumber:         1,
				ID:                   testutils.TestRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_current_no", 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+"_permitted_no", 1, 10*time.Second)
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
				sc:          testutils.SecureCookie,
				redisClient: redisClient,
				config: &waitingroom.Config{
					EntryDelaySec:      10,
					PermittedAccessSec: 10,
					PermitUnitNumber:   10,
					PermitIntervalSec:  10,
				},
			},
			client: waitingroom.Client{
				ID:                   testutils.TestRandomString(20),
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), key+"_current_no", 1, 10*time.Second)
				redisClient.SetEX(context.Background(), key+"_permitted_no", 1, 10*time.Second)
				redisClient.ZAdd(context.Background(), "queue-whitelist", &redis.Z{Member: key, Score: 1})
				redisClient.Expire(context.Background(), "queue-whitelist", 10*time.Second)
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
			repo := repository.NewWaitingroomRepository(redisClient)
			wr := waitingroom.NewWaitingroom(tt.fields.config, repo)
			p := &queueHandler{
				sc:     tt.fields.sc,
				config: tt.fields.config,
				wr:     wr,
			}

			domain := testutils.TestRandomString(20)
			c, rec := testutils.TestContext("/", http.MethodPost, map[string]string{})
			c.SetPath("/queues/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(domain)
			defer rec.Result().Body.Close()
			encoded, err := testutils.SecureCookie.Encode(waitingroom.ClientCookieKey, tt.client)
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
				testutils.SecureCookie.Decode(waitingroom.ClientCookieKey,
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
