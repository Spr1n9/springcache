package connect

import (
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"golang.org/x/net/context"
	"log"
	"time"
)

var (
	//defaultEndpoints    = []string{"10.0.0.166:2379"}
	defaultTimeout      = 3 * time.Second
	defaultLeaseExpTime = 10
)

type Etcd struct {
	EtcdCli *clientv3.Client
	leaseId clientv3.LeaseID // 租约ID
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewEtcd(endpoints []string) (*Etcd, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultTimeout,
	})
	if err != nil {
		log.Println("create etcd register err:", err)
		return nil, err
	}
	// defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	svr := &Etcd{
		EtcdCli: client,
		ctx:     ctx,
		cancel:  cancel,
	}
	return svr, nil
}

// CreateLease 为注册在etcd上的节点创建租约。 由于服务端无法保证自身是一直可用的，可能会宕机，所以与etcd的租约是有时间期限的，租约一旦过期，服务端存储在etcd上的服务地址信息就会消失。
// 另一方面，如果服务端是正常运行的，etcd中的地址信息又必须存在，因此发送心跳检测，一旦发现etcd上没有自己的服务地址时，请求重新添加（续租）。
func (s *Etcd) CreateLease(expireTime int) error {

	res, err := s.EtcdCli.Grant(s.ctx, int64(expireTime))
	if err != nil {
		return err
	}
	s.leaseId = res.ID
	log.Println("create lease success:", s.leaseId)
	return nil

}

// BindLease 把服务和对应的租约绑定
func (s *Etcd) BindLease(server string, addr string) error {

	_, err := s.EtcdCli.Put(s.ctx, server, addr, clientv3.WithLease(s.leaseId))
	if err != nil {
		return err
	}
	return nil
}

func (s *Etcd) KeepAlive() error {
	log.Println("keep alive start")
	log.Println("s.leaseId:", s.leaseId)
	KeepRespChan, err := s.EtcdCli.KeepAlive(context.Background(), s.leaseId)
	if err != nil {
		log.Println("keep alive err:", err)
		return err
	}
	go func() {
		for {
			for KeepResp := range KeepRespChan {
				if KeepResp == nil {
					fmt.Println("keep alive is stop")
					return
				} else {
					fmt.Println("keep alive is ok")
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return nil
}

//func (s *Etcd) KeepAlive() error {
//	log.Println("keep alive start")
//	log.Println("s.leaseId:", s.leaseId)
//	KeepRespChan, err := s.EtcdCli.KeepAlive(context.Background(), s.leaseId)
//	if err != nil {
//		log.Println("keep alive err:", err)
//		return err
//	}
//	go func() {
//		for {
//			select {
//			case KeepResp := <-KeepRespChan:
//				if KeepResp == nil {
//					fmt.Println("keep alive is stop")
//					return
//				} else {
//					fmt.Println("keep alive is ok")
//				}
//			}
//			time.Sleep(7 * time.Second)
//		}
//	}()
//	return nil
//}

// RegisterServer 会把serviceName作为key，addr作为value 存储到etcd中
func (s *Etcd) RegisterServer(serviceName, addr string) error {
	// 创建租约
	err := s.CreateLease(defaultLeaseExpTime)
	if err != nil {
		log.Println("create etcd register err:", err)
		return err
	}
	// 绑定租约
	err = s.BindLease(serviceName, addr)
	if err != nil {
		log.Println("bind etcd register err:", err)
		return err
	}
	// 心跳检测
	err = s.KeepAlive()
	if err != nil {
		log.Println("keep alive register err:", err)
		return err
	}
	// 注册服务用于服务发现
	em, err := endpoints.NewManager(s.EtcdCli, serviceName)
	if err != nil {
		log.Println("create etcd register err:", err)
		return err
	}
	return em.AddEndpoint(s.ctx, serviceName+"/"+addr, endpoints.Endpoint{Addr: addr}, clientv3.WithLease(s.leaseId))
}
