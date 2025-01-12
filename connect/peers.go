package connect

import "time"

// peers 是用于rpc交流的模块

// PeerPicker 定义了获取分布式节点的能力,Server
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 定义了从远端获取缓存的能力,Client
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
	Set(group string, key string, value []byte, expire time.Time, ishot bool) error
}
