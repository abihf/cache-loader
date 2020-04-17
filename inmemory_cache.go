package loader

import "sync"

type inMemoryCache struct {
	sync.Map
}

func InMemoryCache() Cache {
	return &inMemoryCache{sync.Map{}}
}

func (c *inMemoryCache) Add(key, value interface{}) {
	c.Map.Store(key, value)
}

func (c *inMemoryCache) Get(key interface{}) (value interface{}, ok bool) {
	return c.Map.Load(key)
}

func (c *inMemoryCache) Remove(key interface{}) {
	c.Map.Delete(key)
}
