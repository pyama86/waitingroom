package waitingroom

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/pyama86/waitingroom/repository"
)

type AccessController struct {
	config      *Config
	cluster     *Cluster
	waitingroom *Waitingroom
}

func NewAccessController(config *Config, redisClient *redis.Client) *AccessController {
	repo := repository.NewWaitingroomRepository(redisClient)
	wr := NewWaitingroom(config, repo)

	clusterRepo := repository.NewClusterRepository(redisClient)
	cluster := NewCluster(clusterRepo)
	return &AccessController{
		config:      config,
		waitingroom: wr,
		cluster:     cluster,
	}
}
func (a *AccessController) Do(ctx context.Context, e *echo.Echo) error {
	members, err := a.waitingroom.GetEnableDomains(ctx)
	if err != nil {
		return err
	}

	for _, m := range members {
		slog.Info("try permit access", "domain", m)

		ok, err := a.waitingroom.IsEnabledQueue(ctx, m)
		if err != nil {
			return err
		}
		if !ok {
			slog.Info(
				"domain is not enabled",
				"domain", m,
			)
			if err := a.waitingroom.Reset(ctx, m); err != nil {
				return err
			}
			continue
		}

		if ok, err := a.cluster.TryUpdatePermittedNumberLock(ctx, m, time.Duration(a.config.PermitIntervalSec)*time.Second); err != nil {
			return err
		} else if ok {
			if err := a.waitingroom.AppendPermitNumber(ctx, m); err != nil {
				return err
			}
		}

	}

	if len(members) > 0 {
		return a.waitingroom.ExtendDomainsTTL(ctx)
	}

	return nil
}
