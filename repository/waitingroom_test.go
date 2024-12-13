package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/pyama86/waitingroom/repository"
	"github.com/pyama86/waitingroom/testutils"
	"github.com/stretchr/testify/assert"
)

func TestWaitingroomRepository(t *testing.T) {
	ctx := context.Background()
	redisClient := testutils.TestRedisClient()
	repo := repository.NewWaitingroomRepository(redisClient)

	t.Run("AppendPermitNumber", func(t *testing.T) {
		err := repo.AppendPermitNumber(ctx, "test_domain", 1, time.Minute)
		assert.NoError(t, err)

		num, err := repo.GetCurrentPermitNumber(ctx, "test_domain")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), num)
	})

	t.Run("SaveLastNumber", func(t *testing.T) {
		err := repo.SaveLastNumber(ctx, "test_domain", 10, time.Minute)
		assert.NoError(t, err)

		num, err := repo.GetLastNumber(ctx, "test_domain")
		assert.NoError(t, err)
		assert.Equal(t, int64(10), num)
	})

	t.Run("PermitClient", func(t *testing.T) {
		err := repo.PermitClient(ctx, "test_client", time.Minute)
		assert.NoError(t, err)

		exists, err := repo.Exists(ctx, "test_client")
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("ExtendCurrentNumberTTL", func(t *testing.T) {
		err := repo.SaveCurrentNumber(ctx, "test_domain", 1, time.Minute)
		assert.NoError(t, err)

		err = repo.ExtendCurrentNumberTTL(ctx, "test_domain", time.Hour)
		assert.NoError(t, err)

		ttl, err := redisClient.TTL(ctx, "test_domain"+"_current_no").Result()
		assert.NoError(t, err)
		assert.Greater(t, ttl, time.Minute)
	})

	t.Run("EnableDomain", func(t *testing.T) {
		err := repo.EnableDomain(ctx, "test_domain", time.Minute)
		assert.NoError(t, err)

		domains, err := repo.GetEnableDomains(ctx, 0, 1)
		assert.NoError(t, err)
		assert.Contains(t, domains, "test_domain")
	})

	t.Run("DisableDomain", func(t *testing.T) {
		err := repo.DisableDomain(ctx, "test_domain")
		assert.NoError(t, err)

		domains, err := repo.GetEnableDomains(ctx, 0, 1)
		assert.NoError(t, err)
		assert.NotContains(t, domains, "test_domain")
	})

	t.Run("AddWhiteListDomain", func(t *testing.T) {
		err := repo.AddWhiteListDomain(ctx, "test_domain")
		assert.NoError(t, err)

		isWhiteList, err := repo.IsWhiteListDomain(ctx, "test_domain")
		assert.NoError(t, err)
		assert.True(t, isWhiteList)
	})

	t.Run("RemoveWhiteListDomain", func(t *testing.T) {
		err := repo.RemoveWhiteListDomain(ctx, "test_domain")
		assert.NoError(t, err)

		isWhiteList, err := repo.IsWhiteListDomain(ctx, "test_domain")
		assert.NoError(t, err)
		assert.False(t, isWhiteList)
	})
}
