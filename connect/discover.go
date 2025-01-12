package connect

import (
	"context"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

// DialPeer 传入etcd客户端和节点名，获取与其的grpc连接
func DialPeer(c *clientv3.Client, service string) (conn *grpc.ClientConn, err error) {
	PeerResolver, err := resolver.NewBuilder(c)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	//log.Println("In discover.DialPeer, try to get conn, service :", service)

	return grpc.DialContext(ctx,
		"etcd:///"+service,
		grpc.WithResolvers(PeerResolver),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

// GetAddrByName 是根据服务名在etcd中进行服务发现并返回对应的ip地址
func GetAddrByName(c *clientv3.Client, name string) (addr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	//log.Printf("debug, In discover.GetAddrByName, after get ctx")
	resp, err := c.Get(ctx, name)
	if err != nil {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}

//func CheckIf
