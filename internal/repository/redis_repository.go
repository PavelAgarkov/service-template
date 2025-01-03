package repository

import (
	"context"
	"service-template/internal"
	"service-template/pkg"
	"time"
)

const RedisRepositoryLabel = "redis_repository"

type RedisRepository struct {
	locator internal.LocatorInterface
}

func NewRedisRepository() *RedisRepository {
	return &RedisRepository{}
}

func (repo *RedisRepository) SetServiceLocator(container internal.LocatorInterface) {
	repo.locator = container
}

func (repo *RedisRepository) GetServiceLocator() internal.LocatorInterface {
	return repo.locator
}

func (repo *RedisRepository) SetAppName(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	redis := repo.locator.Get(pkg.RedisClientService).(*pkg.RedisClient)
	redis.Client.Set(ctx, key, value, expiration)
	return nil
}
