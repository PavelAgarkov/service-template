package main

import (
	"context"
	"fmt"
	"hash/crc32"
	"sync"
	"time"
)

type Cache interface {
	Set(k, v string, ttl time.Duration)
	Get(k string) (v string, ok bool)
}

// cacheEntry хранит значение и время истечения срока действия.
// Если expiration.IsZero(), то значение не истекает.
type cacheEntry struct {
	value      string
	expiration time.Time
}

// shardedCache разделяет данные на несколько шардов (частей)
// для повышения параллельности доступа.
type shardedCache struct {
	shards []map[string]*cacheEntry
	locks  []sync.RWMutex
	// Для остановки фоновой уборки можно добавить канал, если потребуется.
}

// NewShardedCache создаёт новый шардированный кеш.
// Рекомендуется выбирать число шардов больше или кратное числу потоков.
func NewShardedCache(ctx context.Context, numShards int, cleanUpInterval time.Duration) Cache {
	if numShards <= 0 {
		numShards = 1
	}
	c := &shardedCache{
		shards: make([]map[string]*cacheEntry, numShards),
		locks:  make([]sync.RWMutex, numShards),
	}
	// Инициализируем каждую map
	for i := 0; i < numShards; i++ {
		c.shards[i] = make(map[string]*cacheEntry)
	}
	// Запускаем фоновую уборку просроченных элементов
	go c.startCleanup(ctx, cleanUpInterval)
	return c
}

// startCleanup запускает периодическую уборку просроченных значений.
func (c *shardedCache) startCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			fmt.Println("cleanup stopped")
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// cleanup проходит по всем шардам и удаляет просроченные элементы.
func (c *shardedCache) cleanup() {
	now := time.Now()
	for i := range c.shards {
		c.locks[i].Lock()
		for key, entry := range c.shards[i] {
			// Если для записи задано время истечения и оно прошло – удаляем
			if !entry.expiration.IsZero() && now.After(entry.expiration) {
				delete(c.shards[i], key)
			}
		}
		c.locks[i].Unlock()
	}
}

// hashKey вычисляет индекс шарда по ключу, используя crc32.
func (c *shardedCache) hashKey(k string) int {
	h := crc32.ChecksumIEEE([]byte(k))
	fmt.Println("hashKey:", int(h)%len(c.shards))
	return int(h) % len(c.shards)
}

// Set сохраняет значение v по ключу k с временем жизни ttl.
// Если ttl == 0, то значение сохраняется без срока истечения.
func (c *shardedCache) Set(k, v string, ttl time.Duration) {
	idx := c.hashKey(k)
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	entry := &cacheEntry{
		value:      v,
		expiration: exp,
	}
	c.locks[idx].Lock()
	c.shards[idx][k] = entry
	c.locks[idx].Unlock()
}

// Get возвращает значение по ключу k.
// Если значение найдено, но уже просрочено, оно удаляется и возвращается false.
func (c *shardedCache) Get(k string) (string, bool) {
	idx := c.hashKey(k)
	c.locks[idx].RLock()
	entry, ok := c.shards[idx][k]
	c.locks[idx].RUnlock()
	if !ok {
		return "", false
	}
	// Проверяем, истёк ли ttl
	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		// Просроченное значение удаляем (с дополнительной блокировкой)
		c.locks[idx].Lock()
		delete(c.shards[idx], k)
		c.locks[idx].Unlock()
		return "", false
	}
	return entry.value, true
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(1 * time.Second)
	}()
	cache := NewShardedCache(ctx, 4, 5*time.Second)
	cache.Set("key1", "value1", 10*time.Second)
	cache.Set("key2", "value2", 10*time.Second)
	fmt.Println(cache.Get("key1")) // "", false
	fmt.Println(cache.Get("key2")) // "", false
	time.Sleep(11 * time.Second)
	fmt.Println(cache.Get("key1")) // "", false
	fmt.Println(cache.Get("key2")) // "", false

	//requests()
	//rainbow()
}
