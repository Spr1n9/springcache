package springcache

import (
	"SpringCache/connect"
	"SpringCache/singleflight"
	"fmt"
	"github.com/pkg/errors"
	"log"
	"sync"
	"time"
)

var DefaultExpireTime = 30 * time.Second // 设置短过期时间用于测试

// Group 是 SpringCache 最核心的数据结构，负责与用户的交互，并且控制缓存值存储和获取的流程。
type Group struct {
	name      string
	getter    Getter // 获取源数据的接口
	mainCache cache  // 哈希算法本地存储的键值对
	hotCache  cache  // 热点数据
	peers     connect.PeerPicker
	// use singleflight.Group to make sure that
	// each key is only fetched once
	loader *singleflight.Group // 用于控制并发问题
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group) // 全局变量groups，里面记录了已经创建的group
)

func NewGroup(name string, cacheBytes int64, hotcacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("springcache: getter is nil")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		hotCache:  cache{cacheBytes: hotcacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

func (g *Group) RegisterPeers(peers connect.PeerPicker) {
	if g.peers != nil {
		panic("springcache: peer already registered")
	}
	g.peers = peers
}

func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) Get(key string) (*ByteView, error) {
	if key == "" {
		return &ByteView{}, fmt.Errorf("springcache: key is empty")
	}
	if v, ok := g.lookupCache(key); ok {
		log.Println("SpringCache hit")
		return v, nil
	}
	log.Println("SpringCache miss, try to add it")
	return g.Load(key)
}

// 如果缓存没有命中，则将去远程节点进行查询或查询数据库
// Load loads key either by invoking the getter locally or by sending it to another machine.
func (g *Group) Load(key string) (value *ByteView, err error) {
	// 用Do函数封装实际的load操作，保证并发性
	view, err := g.loader.DoOnce(key, func() (interface{}, error) {
		if g.peers != nil {
			log.Println("try to search from peers")
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err != nil {
					log.Println("springcache: get from peer error:", err)
					return nil, err
				}
				return value, nil
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		return view.(*ByteView), nil
	}
	return
}

func (g *Group) getFromPeer(peer connect.PeerGetter, key string) (*ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return nil, err
	}
	return &ByteView{b: bytes}, nil
}

// 在数据库中查到数据后，添加到缓存中
func (g *Group) getLocally(key string) (*ByteView, error) {
	// 这里调用的是创建Group时存储的getter函数
	bytes, err := g.getter.Get(key)
	if err != nil {
		return &ByteView{}, err
	}
	value := &ByteView{b: cloneBytes(bytes), e: time.Now().Add(DefaultExpireTime)}
	g.populateCache(key, value)
	return value, nil
}

// populateCache 将源数据添加到缓存 mainCache
func (g *Group) populateCache(key string, value *ByteView) {
	g.mainCache.add(key, value)
}

func (g *Group) lookupCache(key string) (value *ByteView, ok bool) {
	value, ok = g.mainCache.get(key)
	if ok {
		return
	}
	value, ok = g.hotCache.get(key)
	return
}

func (g *Group) Set(key string, value *ByteView, ishot bool) error {
	if key == "" {
		return errors.New("key is empty")
	}
	if ishot {
		return g.setHotCache(key, value)
	}
	_, err := g.loader.DoOnce(key, func() (interface{}, error) {
		if peer, ok := g.peers.PickPeer(key); ok {
			err := g.setFromPeer(peer, key, value, ishot)
			if err != nil {
				log.Println("springcache: set from peer error:", err)
				return nil, err
			}
			return value, nil
		}
		// 如果 ！ok，则说明选择到当前节点
		g.mainCache.add(key, value)
		return value, nil
	})
	return err
}

func (g *Group) setFromPeer(peer connect.PeerGetter, key string, value *ByteView, ishot bool) error {
	return peer.Set(g.name, key, value.ByteSlice(), value.Expire(), ishot)
}

// setHotCache 设置热点缓存
func (g *Group) setHotCache(key string, value *ByteView) error {
	if key == "" {
		return errors.New("key is empty")
	}
	g.loader.DoOnce(key, func() (interface{}, error) {
		g.hotCache.add(key, value)
		log.Printf("SpringCache set hot cache %v \n", value.ByteSlice())
		return nil, nil
	})
	return nil
}
