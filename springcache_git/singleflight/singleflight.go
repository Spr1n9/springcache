package singleflight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *Group) DoOnce(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		// 如果g.m[key]不为空，则说明已经有其他线程在请求该key，则等待其他请求结束后一起返回
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)  // 发起请求前加锁
	g.m[key] = c // 加到map中，表示该key已经在请求
	g.mu.Unlock()

	c.val, c.err = fn() // 执行请求
	c.wg.Done()         // 请求结束

	g.mu.Lock()
	delete(g.m, key) // 删除该key，已经执行完毕
	g.mu.Unlock()

	return c.val, c.err
}
