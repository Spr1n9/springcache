package springcache

import (
	"SpringCache/lru"
	"sync"
)

// 设计一个并发缓存
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// add 使用锁保证数据的一致性,底层调用lru的Add方法调整lru结构
func (c *cache) add(key string, value *ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		if lru.DefaultMaxBytes > c.cacheBytes {
			c.lru = lru.New(lru.DefaultMaxBytes, nil)
		} else {
			c.lru = lru.New(c.cacheBytes, nil)
		}
	}
	c.lru.Add(key, value, value.Expire())
}

// get 加锁,调用底层的Get
func (c *cache) get(key string) (value *ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(*ByteView), ok
	}
	return
}
