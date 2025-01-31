package springcache

import "time"

// A ByteView holds an immutable view of bytes.
// ByteView 用来表示缓存值，是SpringCache的存储单元，它实现了lru的Value接口，所以可以直接在lru里面进行存储
type ByteView struct {
	b []byte
	e time.Time
}

func (v *ByteView) Len() int {
	return len(v.b)
}

// Returns the expire time associated with this view
func (v ByteView) Expire() time.Time {
	return v.e
}

func (v *ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v *ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func NewByteView(b []byte, e time.Time) *ByteView {
	return &ByteView{b: b, e: e}
}
