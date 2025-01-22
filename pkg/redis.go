package pkg

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	RedisClientService        = "redis_client"
	RedisClusterClientService = "redis_cluster_client"
)

type RedisClient struct {
	Client *redis.Client
	logger *zap.Logger
}

type RedisClusterClient struct {
	ClusterClient *redis.ClusterClient
	logger        *zap.Logger
}

func NewRedisClient(options *redis.Options, logger *zap.Logger) *RedisClient {
	return &RedisClient{
		Client: redis.NewClient(options),
		logger: logger,
	}
}

func NewClusterClient(options *redis.ClusterOptions, logger *zap.Logger) *RedisClusterClient {
	return &RedisClusterClient{
		ClusterClient: redis.NewClusterClient(options),
		logger:        logger,
	}
}
