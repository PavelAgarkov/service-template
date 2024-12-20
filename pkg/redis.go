package pkg

import "github.com/redis/go-redis/v9"

const (
	RedisClientService        = "redis_client"
	RedisClusterClientService = "redis_cluster_client"
)

type RedisClient struct {
	Client *redis.Client
}

type RedisClusterClient struct {
	ClusterClient *redis.ClusterClient
}

func NewRedisClient(options *redis.Options) *RedisClient {
	return &RedisClient{
		Client: redis.NewClient(options),
	}
}

func NewClusterClient(options *redis.ClusterOptions) *RedisClusterClient {
	return &RedisClusterClient{
		ClusterClient: redis.NewClusterClient(options),
	}
}

//err = rdb.Client.Set(father, "key", "value_1", 1*time.Minute).Err()
//if err != nil {
//	panic(fmt.Sprintf("Не удалось записать значение в Redis: %v", err))
//}
