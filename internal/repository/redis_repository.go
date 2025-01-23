package repository

import (
	"context"
	"service-template/pkg"
	"time"
)

const RedisRepositoryLabel = "redis_repository"

type RedisRepository struct {
	client *pkg.RedisClient
}

func (repo *RedisRepository) SetClient(client *pkg.RedisClient) {
	repo.client = client
}

func NewRedisRepository(client *pkg.RedisClient) *RedisRepository {
	return &RedisRepository{
		client: client,
	}
}

func (repo *RedisRepository) SetAppName(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	redis := repo.client
	redis.Client.Set(ctx, key, value, expiration)
	return nil
}
