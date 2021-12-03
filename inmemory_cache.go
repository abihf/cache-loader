package loader

import "sync"

type inMemoryCache struct {
	sync.Map
}

func InMemoryCache() CacheDriver {
	return &inMemoryCache{}
}

func (c *inMemoryCache) Add(key, value interface{}) {
	c.Store(key, value)
}

func (c *inMemoryCache) Get(key interface{}) (value interface{}, ok bool) {
	return c.Load(key)
}
