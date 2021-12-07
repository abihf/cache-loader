package loader

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CacheDriver stores the items
// you can use ARCCache or TwoQueueCache from github.com/hashicorp/golang-lru
type CacheDriver interface {
	Add(key interface{}, value interface{})
	Get(key interface{}) (interface{}, bool)
}

// Loader manage items in cache and fetch them if not exist
type Loader[Key, Value any] struct {
	*config
	fn    Fetcher[Key, Value]
	mutex sync.Mutex
	def   Value
}

var defaultContextFactory = func() context.Context {
	return context.Background()
}

// New creates new Loader
func New[Key, Value any](fn Fetcher[Key, Value], ttl time.Duration, options ...Option) *Loader[Key, Value] {
	cfg := &config{
		ttl:    ttl,
		errTtl: ttl,
		driver: &inMemoryCache{},
		cf:     defaultContextFactory,
	}
	for _, o := range options {
		o(cfg)
	}
	return &Loader[Key, Value]{
		config: cfg,
		fn:     fn,
	}
}

// Load the item.
// If it doesn't exist on cache, Loader will call LoadFunc once even when other go routine access the same key.
// If the item is expired, it will return old value while loading new one.
func (l *Loader[Key, Value]) Load(key Key) (v Value, e error) {
	l.mutex.Lock()
	unlocked := false
	unlock := func() {
		if !unlocked {
			unlocked = true
			l.mutex.Unlock()
		}
		if r := recover(); r != nil {
			e = fmt.Errorf("error when loading data: %v", r)
		}
	}

	// just to make sure
	// if driver panics, the mutex will be released
	// and this function return error
	defer unlock()

	iface, ok := l.driver.Get(key)
	if ok {
		unlock()

		if iface == nil {
			return l.def, fmt.Errorf("cache driver returns ok but the value is nil")
		}

		item, ok := iface.(*cacheItem[Value])
		if !ok {
			return l.def, fmt.Errorf("cache driver returns invalid value %v", iface)
		}

		item.mutex.RLock()
		defer item.mutex.RUnlock()

		// if the item is expired and it's not doing refetch
		if item.expire.Before(time.Now()) && atomic.CompareAndSwapInt32(&item.isFetching, 0, 1) {
			go l.refetch(key, item)
		}
		return item.value, item.err
	}

	item := &cacheItem[Value]{isFetching: 0}
	item.mutex.Lock()
	defer item.mutex.Unlock()

	l.driver.Add(key, item)
	unlock()

	value, err := l.fn(l.cf(), key)
	if err != nil {
		item.err = err
		item.updateExpire(l.errTtl)
		return l.def, err
	}
	item.value = value
	item.updateExpire(l.ttl)
	return value, nil
}

func (l *Loader[Key, Value]) refetch(key Key, item *cacheItem[Value]) {
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

type cacheItem[Value any] struct {
	value  Value
	err    error
	expire time.Time

	mutex      sync.RWMutex
	isFetching int32
}

func (i *cacheItem[Value]) updateExpire(ttl time.Duration) {
	newExpire := time.Now().Add(ttl)
	i.expire = newExpire
}
