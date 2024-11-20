package waitingroom

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

func TestNewSite(t *testing.T) {
	type args struct {
		Domain string
		config *Config
	}
	tests := []struct {
		name string
		args args
		want *Site
	}{
		{
			name: "ok",
			args: args{
				Domain: "example.com",
				config: &Config{},
			},
			want: &Site{
				Domain: "example.com",
				config: &Config{
					LogLevel:            "",
					Listener:            "",
					PermittedAccessSec:  0,
					EntryDelaySec:       0,
					QueueEnableSec:      0,
					PermitIntervalSec:   0,
					PermitUnitNumber:    0,
					CacheTTLSec:         0,
					NegativeCacheTTLSec: 0,
				},
				permittedNumberKey:           "example.com_permitted_no",
				currentNumberKey:             "example.com_current_no",
				appendPermittedNumberLockKey: "example.com_permitted_no_lock",
				lastNumberKey:                "example.com_last_no",
				cacheEnableKey:               "example.com_enable_cache",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := testContext("/", http.MethodPost, map[string]string{})

			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.args.config)

			tt.want.ctx = ctx.Request().Context()
			tt.want.cache = cache
			tt.want.redisC = redisClient
			if got := NewSite(ctx.Request().Context(), tt.args.Domain, tt.args.config, redisClient, cache); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSite_appendPermitNumber(t *testing.T) {
	type fields struct {
		Domain             string
		config             *Config
		permittedNumberKey string
		currentNumberKey   string
		lastNumberKey      string
	}
	tests := []struct {
		name       string
		fields     fields
		beforeHook func(string, string, string, *redis.Client)
		wantErr    error
		want       int
		wantTTL    time.Duration
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					QueueEnableSec:   10,
					PermitUnitNumber: 10,
				},

				permittedNumberKey: testRandomString(10),
				currentNumberKey:   testRandomString(10),
				lastNumberKey:      testRandomString(10),
			},
			beforeHook: func(permitKey, currentKey, lastKey string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), permitKey, 10, 5*time.Second)
				redisClient.SetEX(context.Background(), currentKey, 10, 5*time.Second)
			},
			wantErr: nil,
			want:    20,
			wantTTL: 5 * time.Second,
		},
		{
			name: "not enable queue",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
				currentNumberKey:   testRandomString(10),
				lastNumberKey:      testRandomString(10),
			},
			wantErr: redis.Nil,
			wantTTL: 10 * time.Second,
		},
		{
			name: "extend TTL",
			fields: fields{
				config: &Config{
					QueueEnableSec:   30,
					PermitUnitNumber: 10,
				},

				permittedNumberKey: testRandomString(10),
				currentNumberKey:   testRandomString(10),
				lastNumberKey:      testRandomString(10),
			},
			beforeHook: func(permitKey, currentKey, lastKey string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), permitKey, 10, 10*time.Second)
				redisClient.SetEX(context.Background(), currentKey, 21, 10*time.Second)
			},
			wantErr: nil,
			want:    20,
			wantTTL: 30 * time.Second,
		},

		{
			name: "reset if not Increase",
			fields: fields{
				config: &Config{
					QueueEnableSec:   10,
					PermitUnitNumber: 10,
				},

				permittedNumberKey: testRandomString(10),
				currentNumberKey:   testRandomString(10),
				lastNumberKey:      testRandomString(10),
			},
			beforeHook: func(permitKey, currentKey, lastKey string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), permitKey, 10, 5*time.Second)
				redisClient.SetEX(context.Background(), currentKey, 10, 5*time.Second)
				redisClient.SetEX(context.Background(), lastKey, 10, 5*time.Second)
			},
			wantErr: ErrClientNotIncrese,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:             tt.fields.Domain,
				ctx:                context.Background(),
				redisC:             redisClient,
				cache:              cache,
				config:             tt.fields.config,
				permittedNumberKey: tt.fields.permittedNumberKey,
				currentNumberKey:   tt.fields.currentNumberKey,
				lastNumberKey:      tt.fields.lastNumberKey,
			}

			if tt.beforeHook != nil {
				tt.beforeHook(tt.fields.permittedNumberKey, tt.fields.currentNumberKey, tt.fields.lastNumberKey, redisClient)
			}

			e := echo.New()

			err := s.AppendPermitNumber(e)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Site.appendPermitNumber() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr == nil {
				got, err := redisClient.Get(context.Background(), tt.fields.permittedNumberKey).Int()
				if err != nil {
					panic(err)
				}
				if got != tt.want {
					t.Errorf("Site.appendPermitNumber() got = %v, want %v", got, tt.want)
				}

				cn, err := redisClient.Get(context.Background(), tt.fields.currentNumberKey).Int()
				if err != nil {
					panic(err)
				}

				ln, err := redisClient.Get(context.Background(), tt.fields.lastNumberKey).Int()
				if err != nil {
					panic(err)
				}

				if cn != ln {
					t.Errorf("Site.appendPermitNumber() currentNumber = %v, lastNumber %v", cn, ln)
				}

				ttl, err := redisClient.TTL(context.Background(), tt.fields.permittedNumberKey).Result()
				if err != nil {
					panic(err)
				}
				if ttl.Seconds() != tt.wantTTL.Seconds() {
					t.Errorf("Site.appendPermitNumber() ttl = %v, want %v", ttl.Seconds(), tt.fields.config.QueueEnableSec)
				}
			}
		})
	}
}

func TestSite_appendPermitNumberIfGetLock(t *testing.T) {
	type fields struct {
		Domain                       string
		config                       *Config
		permittedNumberKey           string
		currentNumberKey             string
		appendPermittedNumberLockKey string
	}
	tests := []struct {
		name       string
		fields     fields
		wantErr    bool
		beforeHook func(string, string, string, *redis.Client)
		want       int
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					QueueEnableSec:   10,
					PermitUnitNumber: 10,
				},
				currentNumberKey:             testRandomString(10),
				permittedNumberKey:           testRandomString(10),
				appendPermittedNumberLockKey: testRandomString(10),
			},
			beforeHook: func(permitKey, currentKey, lockkey string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), permitKey, 10, 10*time.Second)
				redisClient.SetEX(context.Background(), currentKey, 10, 10*time.Second)
			},
			wantErr: false,
			want:    20,
		},
		{
			name: "can't get lock",
			fields: fields{
				config: &Config{
					QueueEnableSec:   10,
					PermitUnitNumber: 10,
				},
				permittedNumberKey:           testRandomString(10),
				currentNumberKey:             testRandomString(10),
				appendPermittedNumberLockKey: testRandomString(10),
			},
			beforeHook: func(permitKey, currentKey, lockkey string, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), permitKey, 10, 10*time.Second)
				redisClient.SetEX(context.Background(), currentKey, 10, 10*time.Second)
				redisClient.SetNX(context.Background(), lockkey, 10, 10*time.Second)
			},
			wantErr: false,
			want:    10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:                       tt.fields.Domain,
				ctx:                          context.Background(),
				redisC:                       redisClient,
				cache:                        cache,
				config:                       tt.fields.config,
				permittedNumberKey:           tt.fields.permittedNumberKey,
				currentNumberKey:             tt.fields.currentNumberKey,
				appendPermittedNumberLockKey: tt.fields.appendPermittedNumberLockKey,
			}
			if tt.beforeHook != nil {
				tt.beforeHook(tt.fields.permittedNumberKey,
					tt.fields.currentNumberKey,
					tt.fields.appendPermittedNumberLockKey, redisClient)
			}

			e := echo.New()
			if err := s.AppendPermitNumberIfGetLock(e); (err != nil) != tt.wantErr {
				t.Errorf("Site.appendPermitNumberIfGetLock() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				got, err := redisClient.Get(context.Background(), tt.fields.permittedNumberKey).Int()
				if err != nil {
					panic(err)
				}
				if got != tt.want {
					t.Errorf("Site.appendPermitNumberIfGetLock() got = %v, want %v", got, tt.want)
				}
			}

		})
	}
}

func TestSite_Reset(t *testing.T) {
	type fields struct {
		Domain                       string
		config                       *Config
		permittedNumberKey           string
		appendPermittedNumberLockKey string
		currentNumberKey             string
		lastNumberKey                string
	}
	tests := []struct {
		name       string
		fields     fields
		wantErr    bool
		beforeHook func(*Site, *redis.Client)
	}{
		{
			name: "ok",
			fields: fields{
				config:                       &Config{},
				permittedNumberKey:           testRandomString(10),
				currentNumberKey:             testRandomString(10),
				lastNumberKey:                testRandomString(10),
				appendPermittedNumberLockKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), s.permittedNumberKey, 10, 10*time.Second)
				redisClient.SetEX(context.Background(), s.currentNumberKey, 10, 10*time.Second)
				redisClient.SetEX(context.Background(), s.lastNumberKey, 10, 10*time.Second)
				redisClient.SetEX(context.Background(), s.appendPermittedNumberLockKey, 10, 10*time.Second)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:                       tt.fields.Domain,
				ctx:                          context.Background(),
				redisC:                       redisClient,
				cache:                        cache,
				config:                       tt.fields.config,
				currentNumberKey:             tt.fields.currentNumberKey,
				permittedNumberKey:           tt.fields.permittedNumberKey,
				appendPermittedNumberLockKey: tt.fields.appendPermittedNumberLockKey,
			}
			if tt.beforeHook != nil {
				tt.beforeHook(s, redisClient)
			}
			if err := s.Reset(); (err != nil) != tt.wantErr {
				t.Errorf("Site.Reset() error = %v, wantErr %v", err, tt.wantErr)
			}

			for _, k := range []string{
				tt.fields.currentNumberKey,
				tt.fields.permittedNumberKey,
				tt.fields.appendPermittedNumberLockKey,
				tt.fields.lastNumberKey,
			} {
				num, err := redisClient.Exists(context.Background(), k).Uint64()
				if err != nil {
					panic(err)
				}
				if num != 0 {
					t.Errorf("Site.Reset() can't Reset = %v", k)
				}
			}
		})
	}
}

func TestSite_IsEnabledQueue(t *testing.T) {
	type fields struct {
		Domain             string
		config             *Config
		permittedNumberKey string
	}
	tests := []struct {
		name       string
		fields     fields
		want       bool
		wantErr    bool
		beforeHook func(*Site, *redis.Client)
	}{
		{
			name: "ok",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), s.permittedNumberKey, 0, 10*time.Second)
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not exist",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				redisClient.Del(context.Background(), s.permittedNumberKey)
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:             tt.fields.Domain,
				ctx:                context.Background(),
				redisC:             redisClient,
				cache:              cache,
				config:             tt.fields.config,
				permittedNumberKey: tt.fields.permittedNumberKey,
			}
			if tt.beforeHook != nil {
				tt.beforeHook(s, redisClient)
			}

			for _, cache := range []bool{true, false} {
				got, err := s.IsEnabledQueue(cache)
				if (err != nil) != tt.wantErr {
					t.Errorf("Site.IsEnabledQueue() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("Site.IsEnabledQueue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSite_EnableQueue(t *testing.T) {
	type fields struct {
		Domain             string
		config             *Config
		permittedNumberKey string
	}
	tests := []struct {
		name       string
		fields     fields
		c          echo.Context
		beforeHook func(*Site, *redis.Client)
		wantErr    bool
		want       string
	}{
		{
			name: "ok",
			fields: fields{
				Domain: testRandomString(10),
				config: &Config{
					QueueEnableSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			want:    "0",
			wantErr: false,
		},
		{
			name: "has cache",
			fields: fields{
				Domain: testRandomString(10),
				config: &Config{
					QueueEnableSec: 20,
				},
				permittedNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				cacheKey := s.Domain + "_enable_cache"
				s.cache.Set(cacheKey, "1", time.Second*10)
				redisClient.SetNX(s.ctx, s.permittedNumberKey, "10", 0)
				redisClient.Expire(s.ctx, s.permittedNumberKey, time.Duration(10)*time.Second)
				redisClient.ZAdd(s.ctx, EnableDomainKey, &redis.Z{
					Score:  1,
					Member: s.Domain},
				)
			},
			want:    "10",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:             tt.fields.Domain,
				ctx:                context.Background(),
				redisC:             redisClient,
				cache:              cache,
				config:             tt.fields.config,
				permittedNumberKey: tt.fields.permittedNumberKey,
				cacheEnableKey:     tt.fields.Domain + "_enable_cache",
			}

			if tt.beforeHook != nil {
				tt.beforeHook(s, redisClient)
			}
			if err := s.EnableQueue(); (err != nil) != tt.wantErr {
				t.Errorf("Site.EnableQueue() error = %v, wantErr %v", err, tt.wantErr)
			}

			rv := redisClient.Get(context.Background(), tt.fields.permittedNumberKey)
			if rv.Err() != nil {
				t.Errorf("got error %v", rv.Err())
			}

			if rv.Val() != tt.want {
				t.Errorf("miss match value got:%v want:%v", rv.Val(), tt.want)
			}

			ev := redisClient.TTL(context.Background(), tt.fields.permittedNumberKey)
			if ev.Err() != nil {
				t.Errorf("got error %v", ev.Err())
			}
			if ev.Val() != time.Duration(10)*time.Second {
				t.Errorf("got ttl %v", ev.Val())
			}
			val, _ := redisClient.ZScan(context.Background(), EnableDomainKey, 0, tt.fields.Domain, 1).Val()
			if len(val) == 0 {
				t.Errorf("%v is not enabled", tt.fields.Domain)
			}
			if !s.cache.Exists(s.Domain + "_enable_cache") {
				t.Errorf("%v has not cache", tt.fields.Domain)
			}
		})
	}
}

func TestSite_IsPermittedClient(t *testing.T) {
	type fields struct {
		Domain             string
		config             *Config
		permittedNumberKey string
	}
	tests := []struct {
		name       string
		fields     fields
		client     *Client
		want       bool
		wantErr    bool
		beforeHook func(*Client, *Site, *redis.Client)
	}{
		{
			name: "already permit",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
			},
			client: &Client{
				ID:           testRandomString(10),
				SerialNumber: 1,
			},
			beforeHook: func(c *Client, s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), c.ID, 1, 10*time.Second)
			},
			want: true,
		},
		{
			name: "not yet permitted",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
			},
			client: &Client{
				ID:           testRandomString(10),
				SerialNumber: 100,
			},
			want: false,
		},
		{
			name: "positive cache",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
			},
			client: &Client{
				ID:           testRandomString(10),
				SerialNumber: 1,
			},
			beforeHook: func(c *Client, s *Site, redisClient *redis.Client) {
				s.cache.Set(c.ID, "1", time.Second*10)
			},
			want: true,
		},
		{
			name: "negative cache",
			fields: fields{
				config:             &Config{},
				permittedNumberKey: testRandomString(10),
			},
			client: &Client{
				ID:           testRandomString(10),
				SerialNumber: 1,
			},
			beforeHook: func(c *Client, s *Site, redisClient *redis.Client) {
				s.cache.Set(c.ID, "-1", time.Second*10)
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:             tt.fields.Domain,
				ctx:                context.Background(),
				redisC:             redisClient,
				cache:              cache,
				config:             tt.fields.config,
				permittedNumberKey: tt.fields.permittedNumberKey,
			}

			if tt.beforeHook != nil {
				tt.beforeHook(tt.client, s, redisClient)
			}

			got, err := s.IsPermittedClient(tt.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Site.IsPermittedClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Site.IsPermittedClient() = %v, want %v", got, tt.want)
			}
			// for not yet started
			if tt.beforeHook == nil {
				v, _ := s.cache.Get(tt.client.ID)
				if v != "-1" {
					t.Error("Site.IsPermittedClient() not created cache")
				}
			}

		})
	}
}

func TestSite_IncrCurrentNumber(t *testing.T) {
	type fields struct {
		Domain           string
		config           *Config
		currentNumberKey string
	}
	tests := []struct {
		name       string
		fields     fields
		want       int64
		wantErr    bool
		beforeHook func(*Site, *redis.Client)
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				currentNumberKey: testRandomString(10),
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "Incr",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				currentNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), s.currentNumberKey, 1, 10*time.Second)
			},
			want:    2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:           tt.fields.Domain,
				ctx:              context.Background(),
				redisC:           redisClient,
				cache:            cache,
				config:           tt.fields.config,
				currentNumberKey: tt.fields.currentNumberKey,
			}

			if tt.beforeHook != nil {
				tt.beforeHook(s, redisClient)
			}
			got, err := s.IncrCurrentNumber()
			if (err != nil) != tt.wantErr {
				t.Errorf("Site.IncrCurrentNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Site.IncrCurrentNumber() = %v, want %v", got, tt.want)
			}
			ev := redisClient.TTL(context.Background(), tt.fields.currentNumberKey)
			if ev.Err() != nil {
				t.Errorf("got error %v", ev.Err())
			}
			if ev.Val() != time.Duration(10)*time.Second {
				t.Errorf("got ttl %v", ev.Val())
			}

		})
	}
}

func TestSite_currentPermitedNumber(t *testing.T) {
	type fields struct {
		Domain             string
		config             *Config
		permittedNumberKey string
	}
	tests := []struct {
		name       string
		fields     fields
		want       int64
		wantErr    bool
		useCache   bool
		beforeHook func(*Site, *redis.Client)
	}{
		{
			name: "ok(nocache)",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), s.permittedNumberKey, 1, 10*time.Second)
			},
			useCache: false,
			want:     1,
			wantErr:  false,
		},
		{
			name: "ok(cache)",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				s.cache.Set(s.permittedNumberKey, "1", 10*time.Second)
			},
			useCache: true,
			want:     1,
			wantErr:  false,
		},
		{
			name: "nil key(nocache)",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			useCache: false,
			wantErr:  true,
		},
		{
			name: "nil key(usecache)",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			useCache: true,
			wantErr:  true,
		},
		{
			name: "negative cache(usecache)",
			fields: fields{
				config: &Config{
					QueueEnableSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			beforeHook: func(s *Site, redisClient *redis.Client) {
				s.cache.Set(s.permittedNumberKey, "-1", 10*time.Second)
			},
			useCache: true,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:             tt.fields.Domain,
				ctx:                context.Background(),
				redisC:             redisClient,
				cache:              cache,
				config:             tt.fields.config,
				permittedNumberKey: tt.fields.permittedNumberKey,
			}

			if tt.beforeHook != nil {
				tt.beforeHook(s, redisClient)
			}
			got, err := s.CurrentPermitedNumber(tt.useCache)
			if (err != nil) != tt.wantErr {
				t.Errorf("Site.currentPermitNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Site.currentPermitNumber() = %v, want %v", got, tt.want)
			}
			if !tt.useCache && !tt.wantErr {
				ev := redisClient.TTL(context.Background(), tt.fields.permittedNumberKey)
				if ev.Err() != nil {
					t.Errorf("got error %v", ev.Err())
				}
				if ev.Val() != time.Duration(10)*time.Second {
					t.Errorf("got ttl %v", ev.Val())
				}
			}
		})
	}
}

func TestSite_permitClient(t *testing.T) {
	type fields struct {
		Domain             string
		config             *Config
		permittedNumberKey string
	}
	tests := []struct {
		name       string
		fields     fields
		client     *Client
		want       bool
		wantErr    bool
		beforeHook func(*Client, *Site, *redis.Client)
	}{
		{
			name: "permit",
			fields: fields{
				config: &Config{
					PermittedAccessSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			client: &Client{
				ID:           testRandomString(10),
				SerialNumber: 1,
			},
			beforeHook: func(c *Client, s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), s.permittedNumberKey, 10, 10*time.Second)
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "was permit",
			fields: fields{
				config: &Config{
					PermittedAccessSec: 10,
				},
				permittedNumberKey: testRandomString(10),
			},
			client: &Client{
				ID:           testRandomString(10),
				SerialNumber: 11,
			},
			beforeHook: func(c *Client, s *Site, redisClient *redis.Client) {
				redisClient.SetEX(context.Background(), s.permittedNumberKey, 10, 10*time.Second)
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			cache := NewCache(redisClient, tt.fields.config)
			s := &Site{
				Domain:             tt.fields.Domain,
				ctx:                context.Background(),
				redisC:             redisClient,
				cache:              cache,
				config:             tt.fields.config,
				permittedNumberKey: tt.fields.permittedNumberKey,
			}

			if tt.beforeHook != nil {
				tt.beforeHook(tt.client, s, redisClient)
			}

			got, err := s.CheckAndPermitClient(tt.client)
			if got != tt.want {
				t.Errorf("Site.permitClient() = %v, want %v", got, tt.want)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Site.permitClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

func TestSite_isSiteIsInWhitelist(t *testing.T) {
	type fields struct {
		Domain string
		config *Config
	}
	tests := []struct {
		name       string
		fields     fields
		want       bool
		wantErr    bool
		beforeHook func(string, *redis.Client)
	}{
		{
			name: "has not whitelist",
			fields: fields{
				Domain: testRandomString(10),
				config: &Config{
					CacheTTLSec: 10,
				},
			},
			wantErr: false,
			want:    false,
		},
		{
			name: "is in whitelist",
			fields: fields{
				Domain: testRandomString(10),
				config: &Config{
					CacheTTLSec: 10,
				},
			},
			beforeHook: func(domain string, redisC *redis.Client) {
				if err := redisC.ZAdd(context.Background(), WhiteListKey, &redis.Z{Score: 1, Member: domain}).Err(); err != nil {
					panic(err)
				}
			},
			wantErr: false,
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisC := testRedisClient()
			cache := NewCache(redisC, tt.fields.config)
			s := &Site{
				Domain: tt.fields.Domain,
				ctx:    context.Background(),
				redisC: redisC,
				cache:  cache,
				config: tt.fields.config,
			}
			if tt.beforeHook != nil {
				tt.beforeHook(tt.fields.Domain, redisC)
			}
			got, err := s.IsInWhitelist()
			if (err != nil) != tt.wantErr {
				t.Errorf("Site.IsInWhitelist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Site.IsInWhitelist() = %v, want %v", got, tt.want)
			}

			if !cache.Exists(WhiteListKey + tt.fields.Domain) {
				t.Error("Site.IsInWhitelist() have not cache")
			}
		})
	}
}
