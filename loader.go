package loader

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// CacheDriver stores the items
// you can use ARCCache or TwoQueueCache from github.com/hashicorp/golang-lru
type CacheDriver interface {
	Add(key, value interface{})
	Get(key interface{}) (value interface{}, ok bool)
}

// Loader manage items in cache and fetch them if not exist
type Loader struct {
	*config
	mutex sync.Mutex
}

var defaultContextFactory = func() context.Context {
	return context.Background()
}

// New creates new Loader
func New(fn Fetcher, ttl time.Duration, options ...Option) *Loader {
	cfg := &config{
		fn:     fn,
		ttl:    ttl,
		errTtl: ttl,
		driver: &inMemoryCache{},
		cf:     defaultContextFactory,
	}
	for _, o := range options {
		o(cfg)
	}
	return &Loader{
		config: cfg,
		mutex:  sync.Mutex{},
	}
}

// Load the item.
// If it doesn't exist on cache, Loader will call LoadFunc once even when other go routine access the same key.
// If the item is expired, it will return old value while loading new one.
func (l *Loader) Load(key interface{}) (interface{}, error) {
	l.mutex.Lock()
	cached, ok := l.driver.Get(key)
	if ok {
		l.mutex.Unlock()

		item := cached.(*cacheItem)
		item.mutex.RLock()
		defer item.mutex.RUnlock()

		// if the item is expired and it's not doing refetch
		if item.expire.Before(time.Now()) && atomic.CompareAndSwapInt32(&item.isFetching, 0, 1) {
			go l.refetch(key, item)
		}
		return item.value, item.err
	}

	item := &cacheItem{isFetching: 0, mutex: sync.RWMutex{}}
	item.mutex.Lock()
	defer item.mutex.Unlock()

	l.driver.Add(key, item)
	l.mutex.Unlock()

	value, err := l.fn(l.cf(), key)
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

	value, err := l.fn(l.cf(), key)

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

	mutex      sync.RWMutex
	isFetching int32
}

func (i *cacheItem) updateExpire(ttl time.Duration) {
	newExpire := time.Now().Add(ttl)
	i.expire = newExpire
}
