package springcache

// A Getter loads data for a key.
// 设计一个回调函数,缓存未命中时会调用这个函数,去获取源数据
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc implements Getter
// 函数类型实现某一个接口，称之为接口型函数，方便使用者在调用时既能够传入函数作为参数，
// 也能够传入实现了该接口的结构体作为参数。
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}
