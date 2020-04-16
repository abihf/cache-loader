package loader

import (
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// NewLRU creates Loader with lru based cache
func NewLRU(fn LoadFunc, ttl time.Duration, size int) *Loader {
	cache, _ := lru.NewARC(size)
	return New(fn, ttl, cache)
}
