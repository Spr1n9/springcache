package lru

import (
	"container/list"
	"testing"
)

type String string

func (d String) Len() int {
	return len(d)
}
func TestGet(t *testing.T) {
	lru := New(int64(0), nil)
	lru.Add(
		"key1", String("1234"),
	)
	if v, ok := lru.Get("key1"); !ok || string(v.(String)) != "1234" {
		t.Fatalf("cache hit key1=1234 failed")
	}
	if _, ok := lru.Get("key2"); ok {
		t.Fatalf("cache miss key2 failed")
	}
}

func TestCache_RemoveOldest(t *testing.T) {
	type fields struct {
		maxBytes  int64
		nbytes    int64
		ll        *list.List
		cache     map[string]*list.Element
		OnEvicted func(key string, value Value)
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
		{
			"1", fields{
				maxBytes:  3,
				nbytes:    0,
				ll:        new(list.List),
				cache:     make(map[string]*list.Element),
				OnEvicted: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cache{
				maxBytes:  tt.fields.maxBytes,
				nbytes:    tt.fields.nbytes,
				ll:        tt.fields.ll,
				cache:     tt.fields.cache,
				OnEvicted: tt.fields.OnEvicted,
			}
			c.RemoveOldest()
		})
	}
}
