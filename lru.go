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
func (c lruWrapper) Add(key any, value any) {
	c.Cache.Add(key, value)
}

func WithLRU(size int, onEvict ...func(key any, value any)) Option {
	var evict func(key any, value any)
	if len(onEvict) > 0 {
		evict = onEvict[0]
	} else if len(onEvict) > 1 {
		panic("only one onEvict function is allowed")
	}
	cache, err := lru.NewWithEvict(size, evict)
	if err != nil {
		panic(err)
	}
	return WithDriver(&lruWrapper{cache})
}

func WithARC(size int) Option {
	cache, err := lru.NewARC(size)
	if err != nil {
		panic(err)
	}
	return WithDriver(cache)
}

// NewLRU creates Loader with lru based cache
func NewLRU[Key comparable, Value any](fn Fetcher[Key, Value], ttl time.Duration, size int, options ...Option) *Loader[Key, Value] {
	options = append(options, WithLRU(size))
	return New(fn, ttl, options...)
}
