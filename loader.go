package loader

import (
	"sync"
	"sync/atomic"
	"time"
)

// LoadFunc loads the value based on key
type LoadFunc func(key interface{}) (interface{}, error)

// Cache stores the items
// you can use ARCCache or TwoQueueCache from github.com/hashicorp/golang-lru
type Cache interface {
	Add(key, value interface{})
	Get(key interface{}) (value interface{}, ok bool)
	Remove(key interface{})
}

// Loader manage items in cache and fetch them if not exist
type Loader struct {
	fn    LoadFunc
	cache Cache

	ttl    time.Duration
	errTtl time.Duration

	mutex sync.Mutex
}

// New creates new Loader
func New(fn LoadFunc, ttl time.Duration, cache Cache) *Loader {
	return &Loader{
		fn:     fn,
		cache:  cache,
		ttl:    ttl,
		errTtl: 10 * time.Second,

		mutex: sync.Mutex{},
	}
}

func (l *Loader) SetErrorTTL(ttl time.Duration) {
	l.errTtl = ttl
}

// Get the item.
// If it doesn't exist on cache, Loader will call LoadFunc once even when other go routine access the same key.
// If the item is expired, it will return old value while loading new one.
func (l *Loader) Get(key interface{}) (interface{}, error) {
	l.mutex.Lock()
	cached, ok := l.cache.Get(key)
	if ok {
		l.mutex.Unlock()

		item := cached.(*cacheItem)
		item.mutex.Lock()
		defer item.mutex.Unlock()

		// if the item is expired and it's not doing refetch
		if item.expire.Before(time.Now()) && atomic.CompareAndSwapInt32(&item.isFetching, 0, 1) {
			go l.refetch(key, item)
		}
		return item.value, item.err
	}

	item := &cacheItem{isFetching: 0, mutex: sync.Mutex{}}
	item.mutex.Lock()
	defer item.mutex.Unlock()

	l.cache.Add(key, item)
	l.mutex.Unlock()

	value, err := l.fn(key)
	if err != nil {
		item.err = err
		item.updateExpire(l.errTtl)
		return nil, err
	}
	item.value = value
	item.updateExpire(l.ttl)
	return value, nil
}

func (l *Loader) refetch(key interface{}, item *cacheItem) {
	defer atomic.StoreInt32(&item.isFetching, 0)

	value, err := l.fn(key)

	item.mutex.Lock()
	defer item.mutex.Unlock()

	if err != nil {
		item.err = err
		item.updateExpire(l.errTtl)
	} else {
		item.value = value
		item.updateExpire(l.ttl)
	}

}

type cacheItem struct {
	value  interface{}
	err    error
	expire time.Time

	mutex      sync.Mutex
	isFetching int32
}

func (i *cacheItem) updateExpire(ttl time.Duration) {
	newExpire := time.Now().Add(ttl)
	i.expire = newExpire
}
