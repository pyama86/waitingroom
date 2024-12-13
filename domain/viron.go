package waitingroom

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/pyama86/waitingroom/repository"
)

type QueueModel struct {
	wr     *Waitingroom
	config *Config
}
type Queue struct {
	Domain          string `json:"domain" validate:"required,fqdn"`
	CurrentNumber   int64  `json:"current_number" validate:"gte=0"`
	PermitetdNumber int64  `json:"permitted_number" validate:"gte=0"`
}

func NewQueueModel(r *redis.Client, config *Config) *QueueModel {
	repo := repository.NewWaitingroomRepository(r)
	wr := NewWaitingroom(config, repo)

	return &QueueModel{
		config: config,
		wr:     wr,
	}
}
func (q *QueueModel) GetQueues(ctx context.Context, perPage, page int64) ([]Queue, int64, error) {
	domains, err := q.wr.GetEnableDomains(ctx,
		&DomainsParam{
			PerPage: perPage * (page - 1),
			Page:    page * perPage,
		},
	)

	if err != nil {
		return nil, 0, err
	}
	ret := []Queue{}
	for _, domain := range domains {
		cn, err := q.wr.GetCurrentNumber(ctx, domain)
		if err != nil {
			return nil, 0, err
		}
		pn, err := q.wr.GetCurrentPermitNumber(ctx, domain)
		if err != nil {
			return nil, 0, err
		}

		ret = append(ret, Queue{
			CurrentNumber:   cn,
			PermitetdNumber: pn,
			Domain:          domain,
		})
	}
	total, err := q.wr.GetEnableDomainsCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	return ret, total, nil
}

func (q *QueueModel) UpdateQueues(ctx context.Context, m *Queue) error {
	if err := q.wr.ExtendDomainsTTL(ctx); err != nil {
		return err
	}

	if err := q.wr.SaveCurrentNumber(ctx, m.Domain, m.CurrentNumber); err != nil {
		return err
	}

	if err := q.wr.SaveCurrentPermitNumber(ctx, m.Domain, m.PermitetdNumber); err != nil {
		return err
	}

	return nil
}

func (q *QueueModel) CreateQueues(ctx context.Context, m *Queue) error {
	if err := q.wr.EnableQueue(ctx, m.Domain); err != nil {
		return err
	}
	return q.UpdateQueues(ctx, m)
}
func (q *QueueModel) DeleteQueues(ctx context.Context, domain string) error {
	return q.wr.Reset(ctx, domain)
}

type WhiteListModel struct {
	wr *Waitingroom
}

type WhiteList struct {
	Domain string `json:"domain" validate:"required,fqdn"`
}

func NewWhiteListModel(r *redis.Client) *WhiteListModel {
	repo := repository.NewWaitingroomRepository(r)
	wr := NewWaitingroom(&Config{}, repo)
	return &WhiteListModel{
		wr: wr,
	}
}
func (q *WhiteListModel) GetWhiteList(ctx context.Context, perPage, page int64) ([]WhiteList, int64, error) {
	members, err := q.wr.GetWhiteListDomains(ctx,
		&DomainsParam{
			PerPage: perPage * (page - 1),
			Page:    page * perPage,
		},
	)
	if err != nil {
		return nil, 0, err
	}
	ret := []WhiteList{}
	for _, m := range members {
		ret = append(ret, WhiteList{Domain: m})
	}

	total, err := q.wr.GetWhiteListDomainsCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	return ret, total, nil
}

func (q *WhiteListModel) CreateWhiteList(ctx context.Context, domain string) error {
	return q.wr.AddWhiteListDomain(ctx, domain)
}

func (q *WhiteListModel) DeleteWhiteList(ctx context.Context, domain string) error {
	return q.wr.RemoveWhiteListDomain(ctx, domain)
}
