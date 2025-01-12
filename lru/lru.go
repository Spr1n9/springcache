// 这里实现lru 算法
package lru

import (
	"container/list"
	"math/rand"
	"time"
)

var DefaultMaxBytes int64 = 10
var DefaultExpireRandom time.Duration = 3 * time.Minute

type NowFunc func() time.Time

var nowFunc NowFunc = time.Now

type Cache struct {
	maxBytes  int64                         // 允许使用的最大内存
	nbytes    int64                         // 当前已经使用的内存
	ll        *list.List                    // 双向队列,用于lru算法
	cache     map[string]*list.Element      // 实际保存键值的缓存
	OnEvicted func(key string, value Value) // 当节点被删除时可以选择性调用回调函数

	// Now is the Now() function the cache will use to determine
	// the current time which is used to calculate expired values
	// Defaults to time.Now()
	Now NowFunc
	//
	ExpireRandom time.Duration
}

type entry struct {
	key     string
	value   Value
	expire  time.Time // 过期时间
	addTime time.Time
}

type Value interface {
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:     maxBytes,
		ll:           list.New(),
		cache:        make(map[string]*list.Element),
		OnEvicted:    onEvicted,
		Now:          nowFunc,
		ExpireRandom: DefaultExpireRandom,
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}

// Get 是用于处理lru逻辑的函数,当请求到某个key就会把他置为队尾
func (c *Cache) Get(key string) (value Value, ok bool) {
	// 如果在缓存中找到对应的节点，则把他移动到队尾
	if ele, ok := c.cache[key]; ok {
		// ll.Value 是一个interface{}类型，可以存储任何类型的数据
		// 所以如果之前链表里存储的是*entry类型，这里就可以断言为*entry类型
		kv := ele.Value.(*entry)
		// 如果kv过期了，将它们移除缓存
		if kv.expire.Before(time.Now()) {
			c.removeElement(ele)
			return nil, false
		}
		// 如果没有过期，更新键值对的添加实现为现在
		expireTime := kv.expire.Sub(kv.addTime)
		kv.expire = time.Now().Add(expireTime)
		kv.addTime = time.Now()
		// 双向链表作为队列，队首队尾是相对的，在这里约定 front 为队尾
		c.ll.MoveToFront(ele)
		return kv.value, true
	}
	return nil, false
}

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		// 在lru队列中删除这个节点
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		// 在缓存中删除这个节点
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Remove(key string) {
	if ele, ok := c.cache[key]; ok {
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(ele *list.Element) {
	kv := ele.Value.(*entry)
	delete(c.cache, kv.key)
	c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
}

func (c *Cache) Add(key string, value Value, expire time.Time) {
	randDuration := time.Duration(rand.Int63n(int64(c.ExpireRandom)))
	if ele, ok := c.cache[key]; ok {
		// 如果key已经存在则将value替换
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
		kv.expire = expire.Add(randDuration)
	} else {
		newEle := c.ll.PushFront(&entry{key, value, expire.Add(randDuration), time.Now()})
		c.cache[key] = newEle
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}
