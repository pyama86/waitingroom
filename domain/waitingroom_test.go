package waitingroom

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pyama86/waitingroom/repository"
	"github.com/pyama86/waitingroom/testutils"
	"go.uber.org/mock/gomock"
)

func TestWaitingroom_AppendPermitNumber(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		wantErr             error
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(1), nil).Times(1)

				mock.EXPECT().GetCurrentPermitNumberTTL(context.Background(), domain).Return(time.Second, nil).Times(1)

				mock.EXPECT().GetCurrentNumber(context.Background(), domain).Return(int64(2000), nil).Times(1)
				mock.EXPECT().GetLastNumber(context.Background(), domain).Return(int64(1), nil).Times(1)
				mock.EXPECT().AppendPermitNumber(context.Background(), domain, int64(1000), 600*time.Second).Return(nil)

				mock.EXPECT().ExtendCurrentNumberTTL(context.Background(), domain, 600*time.Second).Return(nil)
				mock.EXPECT().SaveLastNumber(context.Background(), domain, int64(2000), 600*time.Second).Return(nil)

				return mock
			},
			wantErr: nil,
		},

		{
			name: "reset if not Increase",
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(2), nil).Times(1)

				mock.EXPECT().GetCurrentPermitNumberTTL(context.Background(), domain).Return(time.Second, nil).Times(1)

				mock.EXPECT().GetCurrentNumber(context.Background(), domain).Return(int64(1), nil).Times(1)
				mock.EXPECT().GetLastNumber(context.Background(), domain).Return(int64(1), nil).Times(1)

				mock.EXPECT().DisableDomain(context.Background(), domain).Return(nil).Times(1)
				return mock
			},

			wantErr: ErrClientNotIncrese,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := testutils.TestRandomString(10)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, domain)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)

			err := s.AppendPermitNumber(context.Background(), domain)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Waitingroom.AppendPermitNumber() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestWaitingroom_Reset(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		wantErr             bool
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().DisableDomain(context.Background(), domain).Return(nil).Times(1)
				return mock
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			domain := testutils.TestRandomString(10)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, domain)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)

			if err := s.Reset(context.Background(), domain); (err != nil) != tt.wantErr {
				t.Errorf("Waitingroom.Reset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestWaitingroom_IsEnabledQueue(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		wantErr             error
		want                bool
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			want: true,
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(1), nil).Times(1)

				return mock
			},
			wantErr: nil,
		},
		{
			name: "disabled",
			want: false,
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(-1), nil).Times(1)
				return mock
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := testutils.TestRandomString(10)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, domain)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)

			ok, err := s.IsEnabledQueue(context.Background(), domain)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Waitingroom.IsEnabledQueue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.want {
				t.Errorf("Waitingroom.IsEnabledQueue() got = %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestWaitingroom_EnableQueue(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		wantErr             error
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().EnableDomain(context.Background(), domain, 600*time.Second).Return(nil).Times(1)
				return mock
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := testutils.TestRandomString(10)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, domain)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)

			err := s.EnableQueue(context.Background(), domain)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Waitingroom.EnableQueue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWaitingroom_IsPermittedClient(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		client              *Client
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		wantErr             error
		want                bool
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			client: &Client{
				ID:           testutils.TestRandomString(10),
				SerialNumber: 1,
			},

			waitingroomRepoMock: func(ctrl *gomock.Controller, id string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().Exists(context.Background(), id).Return(true, nil).Times(1)
				return mock
			},
			wantErr: nil,
			want:    true,
		},
		{
			name: "not yet",
			fields: fields{
				config: &Config{
					PermitUnitNumber: 1000,
					QueueEnableSec:   600,
				},
			},
			client: &Client{
				ID:           testutils.TestRandomString(10),
				SerialNumber: 1,
			},

			waitingroomRepoMock: func(ctrl *gomock.Controller, id string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().Exists(context.Background(), id).Return(false, nil).Times(1)
				return mock
			},
			wantErr: nil,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, tt.client.ID)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)
			ok, err := s.IsPermittedClient(context.Background(), tt.client)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Waitingroom.IsPermittedClient() error = %v, wantErr %v", err, tt.wantErr)
			}

			if ok != tt.want {
				t.Errorf("Waitingroom.IsPermittedClient() got = %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestWaitingroom_CheckAndPermitClient(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		client              *Client
		waitingroomRepoMock func(*gomock.Controller, string, string) *repository.MockWaitingroomRepositoryer
		wantErr             error
		want                bool
	}{
		{
			name: "ok",
			fields: fields{
				config: &Config{
					PermitUnitNumber:   1000,
					QueueEnableSec:     600,
					PermittedAccessSec: 10,
				},
			},
			client: &Client{
				ID:           testutils.TestRandomString(10),
				SerialNumber: 1,
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain, id string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(100), nil).Times(1)
				mock.EXPECT().PermitClient(context.Background(), id, 10*time.Second).Return(nil).Times(1)
				return mock
			},
			wantErr: nil,
			want:    true,
		},
		{
			name: "serial number is null",
			fields: fields{
				config: &Config{
					PermitUnitNumber:   1000,
					QueueEnableSec:     600,
					PermittedAccessSec: 10,
				},
			},
			client: &Client{
				ID:           testutils.TestRandomString(10),
				SerialNumber: 0,
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain, id string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(100), nil).Times(1)
				return mock
			},
			wantErr: nil,
			want:    false,
		},
		{
			name: "under permit number",
			fields: fields{
				config: &Config{
					PermitUnitNumber:   1000,
					QueueEnableSec:     600,
					PermittedAccessSec: 10,
				},
			},
			client: &Client{
				ID:           testutils.TestRandomString(10),
				SerialNumber: 1,
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain, id string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(0), nil).Times(1)
				return mock
			},
			wantErr: nil,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := testutils.TestRandomString(10)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, domain, tt.client.ID)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)
			ok, err := s.CheckAndPermitClient(context.Background(), domain, tt.client)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Waitingroom.CheckAndPermitClient() error = %v, wantErr %v", err, tt.wantErr)
			}

			if ok != tt.want {
				t.Errorf("Waitingroom.CheckAndPermitClient() got = %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestWaitingroom_isWaitingroomIsInWhitelist(t *testing.T) {
	type fields struct {
		config *Config
	}
	tests := []struct {
		name                string
		fields              fields
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		want                bool
		wantErr             bool
	}{
		{
			name: "has not whitelist",
			fields: fields{
				config: &Config{
					CacheTTLSec: 10,
				},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().IsWhiteListDomain(context.Background(), domain).Return(false, nil).Times(1)
				return mock
			},
			wantErr: false,
			want:    false,
		},
		{
			name: "is in whitelist",
			fields: fields{
				config: &Config{
					CacheTTLSec: 10,
				},
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().IsWhiteListDomain(context.Background(), domain).Return(true, nil).Times(1)
				return mock
			},
			wantErr: false,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			domain := testutils.TestRandomString(10)
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, domain)
			s := NewWaitingroom(tt.fields.config, waitingroomRepoMock)
			got, err := s.IsInWhitelist(context.Background(), domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("Waitingroom.IsInWhitelist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Waitingroom.IsInWhitelist() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWaitingroom_AssignSerialNumber(t *testing.T) {
	type fields struct {
		SerialNumber         int64
		ID                   string
		TakeSerialNumberTime int64
		domain               string
	}
	tests := []struct {
		name                string
		fields              fields
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		want                int64
		wantErr             bool
	}{
		{
			name: "ok",
			fields: fields{
				SerialNumber:         0,
				ID:                   "dummy",
				TakeSerialNumberTime: time.Now().Unix() - 1,
				domain:               testutils.TestRandomString(20),
			},
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().IncrCurrentNumber(context.Background(), domain, 600*time.Second).Return(int64(1), nil).Times(1)
				return mock
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
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				return mock // No interaction expected
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
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				return mock
			},
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, tt.fields.domain)
			s := NewWaitingroom(&Config{
				QueueEnableSec: 600,
			}, waitingroomRepoMock)

			client := &Client{
				SerialNumber:         tt.fields.SerialNumber,
				ID:                   tt.fields.ID,
				TakeSerialNumberTime: tt.fields.TakeSerialNumberTime,
			}

			got, err := s.AssignSerialNumber(context.Background(), tt.fields.domain, client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Waitingroom.AssignSerialNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Waitingroom.AssignSerialNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
