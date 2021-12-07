package loader

import (
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// lruWrapper wraps hashicorp's lru cache object, so it's compatible with loader cache
type lruWrapper struct {
	*lru.Cache
}

// Add item to cache
func (c *lruWrapper) Add(key interface{}, value interface{}) {
	c.Cache.Add(key, value)
}

// NewLRU creates Loader with lru based cache
func NewLRU[Key, Value any](fn Fetcher[Key, Value], ttl time.Duration, size int, options ...Option) *Loader[Key, Value] {
	cache, err := lru.New(size)
	if err != nil {
		panic(err)
	}
	options = append(options, WithDriver(&lruWrapper{cache}))
	return New(fn, ttl, options...)
}
