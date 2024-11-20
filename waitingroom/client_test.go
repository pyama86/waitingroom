package waitingroom

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/securecookie"
)

var secureCookie = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32),
)

func TestClient_CanTakeSerialNumber(t *testing.T) {
	type fields struct {
		SerialNumber         int64
		ID                   string
		TakeSerialNumberTime int64
		secureCookie         *securecookie.SecureCookie
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "ok",
			fields: fields{
				SerialNumber:         0,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			want: true,
		},
		{
			name: "already has a number",
			fields: fields{
				SerialNumber:         1,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			want: false,
		},
		{
			name: "not reached time",
			fields: fields{
				SerialNumber:         0,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() + 10,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				SerialNumber:         tt.fields.SerialNumber,
				ID:                   tt.fields.ID,
				TakeSerialNumberTime: tt.fields.TakeSerialNumberTime,
				secureCookie:         tt.fields.secureCookie,
			}
			if got := c.canTakeSerialNumber(); got != tt.want {
				t.Errorf("Client.CanTakeSerialNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClientByContext(t *testing.T) {
	tests := []struct {
		name         string
		domain       string
		want         *Client
		cookieClient *Client
		secureCookie *securecookie.SecureCookie
		wantErr      bool
	}{
		{
			name: "ok",
			want: &Client{
				domain:       "example.com",
				secureCookie: secureCookie,
				ID:           "dummy id",
				SerialNumber: 1,
			},
			cookieClient: &Client{
				ID:           "dummy id",
				SerialNumber: 1,
			},
			domain:  "example.com",
			wantErr: false,
		},
		{
			name: "not present cookie",
			want: &Client{
				domain:       "example.com",
				secureCookie: secureCookie,
			},
			domain:  "example.com",
			wantErr: false,
		},
		{
			name: "don't decode secure cookie",
			want: &Client{
				domain:       "example.com",
				secureCookie: secureCookie,
				ID:           "dummy id",
				SerialNumber: 1,
			},
			cookieClient: &Client{
				ID:           "dummy id",
				SerialNumber: 1,
			},
			domain:  "example.com",
			wantErr: false,
		},
		{
			name: "not present cookie",
			cookieClient: &Client{
				ID:           "dummy id",
				SerialNumber: 1,
			},
			secureCookie: securecookie.New(
				securecookie.GenerateRandomKey(16),
				securecookie.GenerateRandomKey(16),
			),
			domain:  "example.com",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		ctx, _ := testContext("/", http.MethodPost, map[string]string{})
		ctx.SetPath("/queues/:domain")
		ctx.SetParamNames("domain")
		ctx.SetParamValues(tt.domain)
		ctx.Request().Host = tt.domain

		if tt.cookieClient != nil {
			if tt.secureCookie == nil {
				tt.secureCookie = secureCookie
			}
			encoded, err := tt.secureCookie.Encode(ClientCookieKey, tt.cookieClient)
			if err != nil {
				panic(err)
			}

			ctx.Request().AddCookie(&http.Cookie{
				Name:     ClientCookieKey,
				Value:    encoded,
				MaxAge:   10,
				Domain:   tt.domain,
				Path:     "/",
				Secure:   true,
				HttpOnly: true,
			})
		}

		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClientByContext(ctx, secureCookie)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClientByContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClientByContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_fillSerialNumber(t *testing.T) {
	redisClient := testRedisClient()
	type fields struct {
		SerialNumber         int64
		ID                   string
		TakeSerialNumberTime int64
		domain               string
	}
	tests := []struct {
		name    string
		fields  fields
		site    *Site
		want    int64
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				SerialNumber:         0,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
				domain:               testRandomString(20),
			},
			site: &Site{
				redisC: redisClient,
				config: &Config{
					QueueEnableSec: 10,
				},
				ctx:              context.Background(),
				currentNumberKey: testRandomString(20),
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "already have number",
			fields: fields{
				SerialNumber:         2,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			want:    2,
			wantErr: false,
		},
		{
			name: "first access",
			fields: fields{
				SerialNumber: 0,
				ID:           "",
			},
			site: &Site{
				config: &Config{
					QueueEnableSec: 10,
				},
			},
			want:    0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				SerialNumber:         tt.fields.SerialNumber,
				ID:                   tt.fields.ID,
				TakeSerialNumberTime: tt.fields.TakeSerialNumberTime,
				secureCookie:         secureCookie,
				domain:               tt.fields.domain,
			}
			got, err := c.FillSerialNumber(tt.site)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.fillSerialNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if c.TakeSerialNumberTime == 0 {
				t.Error("Client.fillSerialNumber() take serial number time is zero")
			}
			if got != tt.want {
				t.Errorf("Client.fillSerialNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_saveToCookie(t *testing.T) {
	type fields struct {
		SerialNumber         int64
		ID                   string
		TakeSerialNumberTime int64
		secureCookie         *securecookie.SecureCookie
		domain               string
	}
	tests := []struct {
		name    string
		want    Client
		fields  fields
		config  *Config
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				SerialNumber:         1,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
				domain:               "example.com",
			},
			want: Client{
				SerialNumber:         1,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
			},
			config: &Config{
				PermittedAccessSec: 10,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fields.secureCookie == nil {
				tt.fields.secureCookie = secureCookie
			}

			c := &Client{
				SerialNumber:         tt.fields.SerialNumber,
				ID:                   tt.fields.ID,
				TakeSerialNumberTime: tt.fields.TakeSerialNumberTime,
				secureCookie:         tt.fields.secureCookie,
				domain:               tt.fields.domain,
			}
			ctx, rec := testContext("/", http.MethodPost, map[string]string{})
			if err := c.SaveToCookie(ctx, tt.config); (err != nil) != tt.wantErr {
				t.Errorf("Client.saveToCookie() error = %v, wantErr %v", err, tt.wantErr)
			}

			parser := &http.Request{Header: http.Header{"Cookie": rec.Header()["Set-Cookie"]}}
			cookie, _ := parser.Cookie(ClientCookieKey)
			got := Client{}
			secureCookie.Decode(ClientCookieKey,
				cookie.Value,
				&got)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestClient_saveToCookie() = %v, want %v", got, tt.want)
			}
		})
	}
}
