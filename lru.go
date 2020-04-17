package loader

import (
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// LRUCache wraps hashicorp's lru cache object, so it's compatible with loader cache
type LRUCache struct {
	*lru.Cache
}

// Add item to cache
func (c *LRUCache) Add(key, value interface{}) {
	c.Cache.Add(key, value)
}

// Remove item from cache
func (c *LRUCache) Remove(key interface{}) {
	c.Cache.Remove(key)
}

// NewLRU creates Loader with lru based cache
func NewLRU(fn LoadFunc, ttl time.Duration, size int) *Loader {
	cache, err := lru.New(size)
	if err != nil {
		panic(err)
	}
	return New(fn, ttl, &LRUCache{cache})
}
