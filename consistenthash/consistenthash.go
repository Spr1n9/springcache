package consistenthash

import (
	"crypto/md5"
	"fmt"
	"github.com/segmentio/fasthash/fnv1"
	"sort"
	"strconv"
	"sync"
)

type Hash func(data []byte) uint64

type Map struct {
	sync.Mutex
	hash     Hash           // 所使用的哈希算法
	replicas int            // 虚拟节点倍数
	keys     []int          // 存储一致性哈希的真实和虚拟节点的数组的哈希环
	hashMap  map[int]string // 虚拟节点与真实节点的映射表
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	// 如果没有选择哈希算法，则使用默认的fnv1算法
	if m.hash == nil {
		m.hash = fnv1.HashBytes64
		// m.hash = crc32.ChecksumIEEE
	}
	return m
}

// AddNodes add some nodes to hash
func (m *Map) AddNodes(keys ...string) {
	m.Lock()
	defer m.Unlock()
	//log.Println("In consistenthash.AddNodes, keys =", keys)
	for _, key := range keys {
		// 对每个新加的真实节点都创建一些虚拟节点，用于解决一致性哈希中数据倾斜的问题
		for i := 0; i < m.replicas; i++ {
			//log.Printf("当前处理节点是%v，正在添加它的第%v个虚拟节点", key, i)
			hash := int(m.hash([]byte(fmt.Sprintf("%x", md5.Sum([]byte(strconv.Itoa(i)+key)))))) // 通过哈希算法得到虚拟节点的哈希值
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key // 把映射记录到表中 hashMap[虚拟节点哈希]真实节点key
		}
	}
	sort.Ints(m.keys) // 将哈希环的节点排序
	//fmt.Printf("In consistenthash.AddNodes, m.keys = %v, m.hashMap = %v \n", m.keys, m.hashMap)
}

// Get 会根据传入的键去找到存储该键值对的真实节点，并返回真实节点的ip地址
func (m *Map) Get(key string) string {

	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	// 顺时针匹配第一个虚拟节点
	//idx := sort.Search(len(m.keys), func(i int) bool {
	//	return m.keys[i] >= hash
	//})

	// 通过二分查找找到节点
	// 注意这里的j不是 len(m.keys)-1 ,因为我们要让他在查找的哈希值大于环上最大值时
	// 返回结果为len(m.keys)
	i, j := 0, len(m.keys)
	for i < j {
		mid := uint((i + j) >> 1)
		if m.keys[mid] >= hash {
			j = int(mid)
		} else if m.keys[mid] < hash {
			i = int(mid) + 1
		}
	}
	idx := i
	// 有一种特殊情况是 当加入的hash值超过环上最大节点的哈希值时,需要将其指向第一个节点
	// 当这种情况发生时, 如果当前哈希表中有9个节点，那么 idx 会等于 9
	// 所以对idx与9取余, 使得当idx=9 的时候能归并为 0 ,也就是第一个节点
	//log.Println("here, consistenthash.Get, idx: ", idx, ", m.keys[idx%len(m.keys)]:", m.keys[idx%len(m.keys)], ", m.hashMap[m.keys[idx%len(m.keys)]]:", m.hashMap[m.keys[idx%len(m.keys)]])
	return m.hashMap[m.keys[idx%len(m.keys)]]
}

func (m *Map) Remove(key string) {
	m.Lock()
	defer m.Unlock()
	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(fmt.Sprintf("%x", md5.Sum([]byte(strconv.Itoa(i)+key))))))
		idx := sort.SearchInts(m.keys, hash)
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
	}
}
