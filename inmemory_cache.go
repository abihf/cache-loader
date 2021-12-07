package loader

import "sync"

type inMemoryCache struct {
	sync.Map
}

func InMemoryCache() CacheDriver {
	return &inMemoryCache{}
}

func (c *inMemoryCache) Add(key interface{}, value interface{}) {
	c.Store(key, value)
}

func (c *inMemoryCache) Get(key interface{}) (interface{}, bool) {
	return c.Load(key)
}
