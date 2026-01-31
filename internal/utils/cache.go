package utils

import (
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// CacheItem 包装缓存数据和过期时间
type CacheItem struct {
	Data      interface{}
	ExpiresAt time.Time
}

// GlobalCache 全局本地缓存封装
type GlobalCache struct {
	lruCache *lru.Cache[string, CacheItem]
}

var cacheInstance *GlobalCache

// GetCache 获取单例缓存实例
func GetCache() *GlobalCache {
	if cacheInstance == nil {
		// 创建一个容量为 500 的 LRU 缓存
		l, err := lru.New[string, CacheItem](500)
		if err != nil {
			log.Fatalf("Failed to create LRU cache: %v", err)
		}
		cacheInstance = &GlobalCache{
			lruCache: l,
		}
	}
	return cacheInstance
}

// Set 设置缓存，TTL 为过期时间
func (c *GlobalCache) Set(key string, data interface{}, ttl time.Duration) {
	c.lruCache.Add(key, CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	})
}

// Get 获取缓存，若不存在或已过期则返回 nil
func (c *GlobalCache) Get(key string) interface{} {
	val, ok := c.lruCache.Get(key)
	if !ok {
		return nil
	}

	// 检查过期
	if time.Now().After(val.ExpiresAt) {
		c.lruCache.Remove(key)
		return nil
	}

	return val.Data
}

// Delete 删除指定缓存
func (c *GlobalCache) Delete(key string) {
	c.lruCache.Remove(key)
}
