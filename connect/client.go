package connect

import (
	pb "SpringCache/springcachepb"
	"context"
	"fmt"
	"log"
	"time"
)

// client 包是去调用远端的方法
type Client struct {
	Name string
	Etcd *Etcd
}

func newClient(name string, etcd *Etcd) *Client {
	return &Client{name, etcd}
}

func (c *Client) Get(group string, key string) ([]byte, error) {

	// 用etcd进行服务发现, 获得grpc的连接
	conn, err := DialPeer(c.Etcd.EtcdCli, c.Name)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 创建grpc客户端，调用远程peer的get方法
	grpcClient := pb.NewSpringCacheClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := grpcClient.Get(ctx, &pb.GetRequest{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get %s/%s from peer %s", group, key, c.Name)
	}
	log.Println("In client.Get, grpcClient.Get Done, resp :", resp)
	return resp.GetValue(), nil
}

func (c *Client) Set(group string, key string, value []byte, expire time.Time, ishot bool) error {

	// 用etcd进行服务发现, 获得grpc的连接
	conn, err := DialPeer(c.Etcd.EtcdCli, c.Name)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 创建grpc客户端，调用远程peer的get方法
	grpcClient := pb.NewSpringCacheClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := grpcClient.Set(ctx, &pb.SetRequest{
		Group:  group,
		Key:    key,
		Value:  value,
		Expire: expire.Unix(),
		Ishot:  ishot,
	})
	if err != nil {
		log.Println("grpcClient.Set Error:", err)
		return err
	}
	if !resp.GetOk() {
		return fmt.Errorf("grpcClient.Set Failed !")
	}
	return nil
}

// 验证是否实现接口
var _ PeerGetter = (*Client)(nil)
