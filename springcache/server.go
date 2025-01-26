package springcache

import (
	"SpringCache/connect"
	"SpringCache/consistenthash"
	pb "SpringCache/springcachepb"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// server 处理别人发来的请求
var (
	ErrorServerHasStarted = errors.New("server has started, err:")
	ErrorTcpListen        = errors.New("tcp listen error")
	ErrorRegisterEtcd     = errors.New("register etcd error")
	ErrorGrpcServerStart  = errors.New("start grpc server error")
)

var (
	defaultListenAddr = "0.0.0.0:8888"
)

const (
	defaultReplicas = 50
)

type Server struct {
	pb.UnimplementedSpringCacheServer

	status  bool   // 标记服务是否正在运行
	self    string // 标记自己的ip地址
	mu      sync.Mutex
	peers   *consistenthash.Map // 哈希相关
	etcd    *connect.Etcd
	name    string
	clients map[string]*connect.Client // 【节点名】客户端
}

// NewServer 会创建一个grpc服务端，并绑定与etcd进行绑定
func NewServer(serverName, selfAddr string, etcd *connect.Etcd) *Server {

	return &Server{
		self:    selfAddr,
		status:  false,
		peers:   consistenthash.New(defaultReplicas, nil),
		etcd:    etcd,
		clients: make(map[string]*connect.Client),
		name:    serverName,
	}
}

// 实现grpc定义的接口Get，即当远端调用该节点时查找缓存时，返回对应的值
func (s *Server) Get(ctx context.Context, in *pb.GetRequest) (out *pb.GetResponse, err error) {
	groupName, key := in.GetGroup(), in.GetKey()
	group := GetGroup(groupName)
	bytes, err := group.Get(key)
	if err != nil {
		return nil, err
	}
	out = &pb.GetResponse{
		Value: bytes.ByteSlice(),
	}
	return out, nil
}

// 实现grpc定义的接口Set，远端调用该节点设置缓存
func (s *Server) Set(ctx context.Context, in *pb.SetRequest) (out *pb.SetResponse, err error) {
	groupName, key, value, expire := in.GetGroup(), in.GetKey(), in.GetValue(), in.GetExpire()
	ishot := in.GetIshot()
	group := GetGroup(groupName)
	bytes := NewByteView(value, time.Unix(expire, 0))
	out = &pb.SetResponse{
		Ok: false,
	}
	err = group.Set(key, bytes, ishot)
	if err != nil {
		return out, err
	}
	return &pb.SetResponse{Ok: true}, nil
}

func (s *Server) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", s.self, fmt.Sprintf(format, v...))
}

// SetPeers 会把节点名在etcd中进行服务发现，并把获取的ip地址加入到哈希环中，并且把客户端保存到clients这个map中方便后面调用
func (s *Server) SetPeers(names ...string) {

	for _, name := range names {
		//log.Printf("debug, In server.SetPeers, name:", name)
		ip, err := connect.GetAddrByName(s.etcd.EtcdCli, name)
		if err != nil {
			log.Printf("SetPeers err : %v", err)
			return
		}
		//log.Printf("debug, In server.SetPeers, ip:", ip)
		addr := strings.Split(ip, ":")[0]
		s.peers.AddNodes(addr)
		s.clients[addr] = &connect.Client{name, s.etcd}
	}
	//log.Println("SetPeers success, s.clients =", s.clients)
}

// PickPeer 包装了一致性哈希算法的 Get() 方法，根据具体的 key，选择节点，
// 返回节点对应的 rpc服务器。
func (s *Server) PickPeer(key string) (connect.PeerGetter, bool) {
	//s.mu.Lock()
	//defer s.mu.Unlock()
	if peer := s.peers.Get(key); peer != "" {
		ip := strings.Split(s.self, ":")[0]
		if peer == ip {
			log.Println("ops! peek my self! , i am :", peer)
			return nil, false
		}
		s.Log("Pick peer %s", peer)
		return s.clients[peer], true
	}
	return nil, false
}

// 根据key找出移除哈希环上存储该键值对的节点，并移除这个节点
func (s *Server) RemovePeerByKey(key string) {
	peer := s.peers.Get(key)
	s.peers.Remove(peer)
	log.Printf("RemovePeer %s", peer)
}

// StartServer 开启grpc服务，并在etcd上注册
func (s *Server) StartServer() error {
	// -----------------启动服务----------------------
	// 1. 设置status为true 表示服务器已在运行
	// 2. 初始化tcp socket并开始监听
	// 3. 注册rpc服务至grpc 这样grpc收到request可以分发给server处理
	// ----------------------------------------------
	s.mu.Lock()
	if s.status {
		s.mu.Unlock()
		return ErrorServerHasStarted
	}
	// 开启grpc
	lis, err := net.Listen("tcp", defaultListenAddr)
	if err != nil {
		log.Println("listen server error:", err)
		return ErrorTcpListen
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSpringCacheServer(grpcServer, s)

	log.Println("start grpc server:", s.self)
	err = grpcServer.Serve(lis)
	if err != nil {
		log.Println(ErrorGrpcServerStart, "err： ", err)
		return ErrorGrpcServerStart
	}
	s.status = true
	s.mu.Unlock()
	return nil
}

var _ connect.PeerPicker = (*Server)(nil)
