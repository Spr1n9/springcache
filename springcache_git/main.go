package main

import (
	"SpringCache/connect"
	"SpringCache/springcache"
	"flag"
	"fmt"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var store = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// api服务
func startAPIServer(apiAddr string, group *springcache.Group, svr *springcache.Server) {

	getHandle := func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		view, err := group.Get(key)
		if err != nil {
			if err == context.DeadlineExceeded {
				// 如果发现超时，则说明远方节点不可用
				// 删除该节点在哈希环上的记录，并且向数据库请求数据
				svr.RemovePeerByKey(key)
				view, err = group.Load(key)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/octet-stream")
				value := fmt.Sprintf("value=%v\n", string(view.ByteSlice()))
				w.Write([]byte(value))
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		value := fmt.Sprintf("value=%v\n", string(view.ByteSlice()))
		w.Write([]byte(value))
	}
	setPeerHandle := func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		peer := r.FormValue("peer")
		if peer == "" {
			http.Error(w, "peer is not allow empty!", http.StatusInternalServerError)
			return
		}
		//log.Println("debug : In main.setPeerHandler, get peer:", peer)
		svr.SetPeers(peer)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(fmt.Sprintf("set peer %v successful\n", peer)))
	}
	setHandle := func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error ParseForm", http.StatusInternalServerError)
			return
		}
		key := r.FormValue("key")
		value := r.FormValue("value")
		expire := r.FormValue("expire")
		hot := r.FormValue("hot")
		expireTime, err := strconv.Atoi(expire)
		if err != nil {
			w.Write([]byte("请正确设置超时时间\"expire\",单位：分钟"))
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		if expireTime < 0 || expireTime > 4321 {
			w.Write([]byte("过期时间设置有误，单位是分钟，最长过期时间不超过4320分钟（3天）"))
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		ishot := false
		if hot == "true" {
			ishot = true
		}
		if hot != "true" && hot != "false" && hot != "" {
			w.Write([]byte("Invaild Param \"hot\" "))
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		exp := time.Duration(expireTime) * time.Minute
		exptime := time.Now().Add(exp)
		byteView := springcache.NewByteView([]byte(value), exptime)
		//log.Printf("debug, In main.sethandler, key=%v, value=%v, expire=%v, hot=%v\n", key, value, expire, hot)
		if err := group.Set(key, byteView, ishot); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("done\n"))
	}
	http.HandleFunc("/api/get", getHandle)
	http.HandleFunc("/setpeer", setPeerHandle)
	http.HandleFunc("/api/set", setHandle)
	log.Println("fontend server is running at", apiAddr[7:])
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

func main() {

	var (
		addr           = os.Getenv("IP_ADDRESS")
		svrName        = flag.String("name", "", "server name")
		port           = flag.String("port", "8888", "server port")
		peers          = flag.String("peer", "", "peers name")
		etcdAddr       = flag.String("etcd", "127.0.0.1:2379", "etcd address")
		defaultApiAddr = "http://0.0.0.0:9999"
	)
	flag.Parse()
	if *svrName == "" {
		log.Fatal("--name is required")
	}
	if *peers == "" {
		log.Fatal("--peer is required")
	}
	if !strings.Contains(*peers, *svrName) {
		log.Fatal("--peers must contains " + *svrName)
	}
	if addr == "" {
		log.Fatal("please set env IP_ADDRESS")
	}

	// 开启代码时要记得 --name，--peer, 设置好 env IP_ADDRESS

	// 新建cache实例
	group := springcache.NewGroup("scores", 2<<10, 2<<7, springcache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Printf("Searching the \"%v\" from databases", key)
			if v, ok := store[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	// 新建一个etcd的客户端
	etcd, err := connect.NewEtcd([]string{*etcdAddr})
	if err != nil {
		log.Println("etcd connect err:", err)
		panic(err)
	}

	log.Println("server name: ", *svrName)
	address := fmt.Sprintf("%s:%s", addr, *port)
	err = etcd.RegisterServer(*svrName, address)
	if err != nil {
		log.Fatal("register server error:", err)
	}
	log.Println("register server is Done")

	log.Println("grpc server address :", address)
	// 新建grpc Server
	svr := springcache.NewServer(*svrName, address, etcd)

	// 把节点加入到哈希环中
	// 检查其余节点是否在etcd中注册，如果没有则等待
	peer := strings.Split(*peers, ",")
	if len(peer) != 1 {
		timer := 0
		log.Println("wait other server to register")
		done := make(chan bool, 1)
		go func() {
			for {
				if IfAllRegistered(etcd, peer) {
					break
				}
				time.Sleep(2 * time.Second)
				timer++
				if timer > 15 {
					log.Fatal("other svc doesn't register, please check and try agian later")
				}
			}
			done <- true
		}()
		<-done
	}
	log.Println("other servers are registered")
	svr.SetPeers(peer...)
	// 把服务与group绑定
	group.RegisterPeers(svr)
	// 开启api服务
	go startAPIServer(defaultApiAddr, group, svr)

	// 开启服务
	err = svr.StartServer()
	if err != nil {
		log.Println("grpc server start err:", err)
		panic(err)
	}

}

func IfAllRegistered(etcd *connect.Etcd, peer []string) bool {
	for _, v := range peer {
		resp, err := etcd.EtcdCli.Get(context.Background(), v)
		if err != nil || len(resp.Kvs) == 0 {
			return false
		}
	}
	return true
}
