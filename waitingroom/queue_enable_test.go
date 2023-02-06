package waitingroom

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-redis/redis/v8"
)

func TestQueueEnable_Do(t *testing.T) {
	tests := []struct {
		name        string
		redisClient *redis.Client
		config      *Config
		wantErr     bool
		wantStatus  int
	}{
		{
			name: "ok",
			config: &Config{
				QueueEnableSec: 600,
			},
			wantErr:    false,
			wantStatus: http.StatusCreated,
		},
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &QueueEnable{
				config:      tt.config,
				redisClient: redisClient,
			}
			c, rec := postContext("/", map[string]string{})
			c.SetPath("/queues/enable/:domain")
			c.SetParamNames("domain")
			c.SetParamValues(tt.name)
			if err := p.Do(c); (err != nil) != tt.wantErr {
				t.Errorf("QueueEnable.Do() error = %v, wantErr %v", err, tt.wantErr)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("status code does not match, expected %d, got %d", tt.wantStatus, rec.Code)
			}
			r := redisClient.SIsMember(c.Request().Context(), enableDomainsKey(), tt.name)
			if r.Err() != nil {
				t.Errorf("got error %v", r.Err())
			}

			if !r.Val() {
				t.Errorf("%s isn't in redis", tt.name)
			}
			rv := redisClient.Get(c.Request().Context(), enableDomainKey(c))
			if rv.Err() != nil {
				t.Errorf("got error %v", rv.Err())
			}

			if rv.Val() != "1" {
				t.Errorf("%s=%s isn't in redis", tt.name, rv.Val())
			}

		})
	}
}

func BenchmarkQueueEnable_Do(b *testing.B) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
	})
	p := &QueueEnable{
		config: &Config{
			QueueEnableSec: 600,
		},
		redisClient: redisClient,
	}

	for i := 0; i < b.N; i++ {
		c, _ := postContext("/", map[string]string{})
		c.SetPath("/queues/enable/:domain")
		c.SetParamNames("domain")
		c.SetParamValues(fmt.Sprintf("%d", i))
		if err := p.Do(c); err != nil {
			b.Errorf("QueueEnable.Do() error = %v", err)
		}
	}
}
