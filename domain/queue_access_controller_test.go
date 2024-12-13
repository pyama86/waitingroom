package waitingroom

import (
	"context"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pyama86/waitingroom/repository"
	"github.com/pyama86/waitingroom/testutils"
	"go.uber.org/mock/gomock"
)

func TestAccessController_Do(t *testing.T) {
	tests := []struct {
		ctx                 context.Context
		waitingroomRepoMock func(*gomock.Controller, string) *repository.MockWaitingroomRepositoryer
		clusterRepoMock     func(*gomock.Controller) *repository.MockClusterRepositoryer
		name                string
		domain              string
		wantErr             bool
	}{
		{
			name:   "ok",
			domain: testutils.TestRandomString(20),
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetEnableDomains(context.Background(), int64(0), int64(-1)).Return([]string{domain}, nil)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(1), nil).Times(2)

				mock.EXPECT().GetCurrentPermitNumberTTL(context.Background(), domain).Return(time.Second, nil).Times(1)

				mock.EXPECT().GetCurrentNumber(context.Background(), domain).Return(int64(2000), nil).Times(1)
				mock.EXPECT().GetLastNumber(context.Background(), domain).Return(int64(1), nil).Times(1)
				mock.EXPECT().AppendPermitNumber(context.Background(), domain, int64(1000), 600*time.Second).Return(nil)

				mock.EXPECT().ExtendCurrentNumberTTL(context.Background(), domain, 600*time.Second).Return(nil)
				mock.EXPECT().SaveLastNumber(context.Background(), domain, int64(2000), 600*time.Second).Return(nil)
				mock.EXPECT().ExtendDomainsTTL(context.Background(), 600*time.Second*2).Return(nil)

				return mock
			},
			clusterRepoMock: func(ctrl *gomock.Controller) *repository.MockClusterRepositoryer {
				mock := repository.NewMockClusterRepositoryer(ctrl)
				mock.EXPECT().GetLockforPermittedNumber(context.Background(), gomock.Any(), gomock.Any()).Return(true, nil)
				return mock
			},
		},
		{
			name:   "skip update because not enabled domain",
			domain: testutils.TestRandomString(20),
			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetEnableDomains(context.Background(), int64(0), int64(-1)).Return([]string{"unmatch"}, nil)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), "unmatch").Return(int64(-1), nil).Times(1)
				mock.EXPECT().DisableDomain(context.Background(), "unmatch").Return(nil).Times(1)
				mock.EXPECT().ExtendDomainsTTL(context.Background(), 600*time.Second*2).Return(nil)
				return mock
			},
			clusterRepoMock: func(ctrl *gomock.Controller) *repository.MockClusterRepositoryer {
				mock := repository.NewMockClusterRepositoryer(ctrl)
				return mock
			},
		},
		{
			name:   "skip update because can't get lock",
			domain: testutils.TestRandomString(20),

			waitingroomRepoMock: func(ctrl *gomock.Controller, domain string) *repository.MockWaitingroomRepositoryer {
				mock := repository.NewMockWaitingroomRepositoryer(ctrl)
				mock.EXPECT().GetEnableDomains(context.Background(), int64(0), int64(-1)).Return([]string{domain}, nil)
				mock.EXPECT().GetCurrentPermitNumber(context.Background(), domain).Return(int64(1), nil).Times(1)
				mock.EXPECT().ExtendDomainsTTL(context.Background(), 600*time.Second*2).Return(nil)

				return mock
			},
			clusterRepoMock: func(ctrl *gomock.Controller) *repository.MockClusterRepositoryer {
				mock := repository.NewMockClusterRepositoryer(ctrl)
				mock.EXPECT().GetLockforPermittedNumber(context.Background(), gomock.Any(), gomock.Any()).Return(false, nil)
				return mock
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			waitingroomRepoMock := tt.waitingroomRepoMock(ctrl, tt.domain)
			clusterRepoMock := tt.clusterRepoMock(ctrl)
			config := &Config{
				PermitUnitNumber: 1000,
				QueueEnableSec:   600,
			}
			a := &AccessController{
				config:      config,
				waitingroom: NewWaitingroom(config, waitingroomRepoMock),
				cluster:     NewCluster(clusterRepoMock),
			}

			e := echo.New()
			if err := a.Do(context.Background(), e); (err != nil) != tt.wantErr {
				t.Errorf("AccessController.Do() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
