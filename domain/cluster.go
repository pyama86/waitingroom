package waitingroom

import (
	"context"
	"time"

	"github.com/pyama86/waitingroom/repository"
)

type Cluster struct {
	repository repository.ClusterRepositoryer
}

func NewCluster(r repository.ClusterRepositoryer) *Cluster {
	return &Cluster{
		repository: r,
	}
}

func (c *Cluster) TryUpdatePermittedNumberLock(ctx context.Context, domain string, ttl time.Duration) (bool, error) {
	return c.repository.GetLockforPermittedNumber(ctx, domain, ttl)
}
