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
func (c *lruWrapper) Add(key, value interface{}) {
	c.Cache.Add(key, value)
}

// NewLRU creates Loader with lru based cache
func NewLRU(fn Fetcher, ttl time.Duration, size int, options ...Option) *Loader {
	cache, err := lru.New(size)
	if err != nil {
		panic(err)
	}
	options = append(options, WithDriver(&lruWrapper{cache}))
	return New(fn, ttl, options...)
}
