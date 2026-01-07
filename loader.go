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
	Add(key any, value any)
	Get(key any) (any, bool)
}

// Fetcher loads the value based on key
type Fetcher[Key comparable, Value any] func(ctx context.Context, key Key) (Value, error)

// Loader manage items in cache and fetch them if not exist
type Loader[Key comparable, Value any] struct {
	*config
	fn  Fetcher[Key, Value]
	def Value

	lock KeyLocker[Key]
}

// New creates new Loader
func New[Key comparable, Value any](fn Fetcher[Key, Value], ttl time.Duration, options ...Option) *Loader[Key, Value] {
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
		lock:   newInMemoryKeyLocker[Key](), // TODO: make it configurable
	}
}

func (l *Loader[Key, Value]) fastLoad(key Key) (Value, error, bool) {
	iface, ok := l.driver.Get(key)
	if !ok {
		return l.def, nil, false
	}
	if iface == nil {
		return l.def, fmt.Errorf("cache driver returns ok but the value is nil"), true
	}

	item, ok := iface.(*cacheItem[Value])
	if !ok {
		return l.def, fmt.Errorf("cache driver returns invalid value %v", iface), true
	}

	item.mutex.RLock()
	defer item.mutex.RUnlock()

	// if the item is expired and it's not doing refetch
	if item.expire.Before(time.Now()) && item.isFetching.CompareAndSwap(false, true) {
		go l.refetch(key, item)
	}

	return item.value, item.err, true
}

// Load the item.
// If it doesn't exist on cache, Loader will call LoadFunc once even when other go routine access the same key.
// If the item is expired, it will return old value while loading new one.
func (l *Loader[Key, Value]) Load(key Key) (Value, error) {
	// fast path
	v, err, ok := l.fastLoad(key)
	if ok {
		return v, err
	}

	unlock := l.lock.Lock(key)

	// double check after get the lock
	v, err, ok = l.fastLoad(key)
	if ok {
		return v, err
	}

	// add a placeholder to avoid other go routine fetching the same key
	item := &cacheItem[Value]{}
	item.isFetching.Store(false)
	item.mutex.Lock()
	defer item.mutex.Unlock()

	l.driver.Add(key, item)
	unlock()

	// fetch the value
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
	defer item.isFetching.Store(false)

	value, err := l.fn(l.cf(), key)

	item.mutex.Lock()
	defer item.mutex.Unlock()

	item.value, item.err = value, err
	if err != nil {
		item.updateExpire(l.errTtl)
	} else {
		item.updateExpire(l.ttl)
	}
}

type cacheItem[Value any] struct {
	value  Value
	err    error
	expire time.Time

	mutex      sync.RWMutex
	isFetching atomic.Bool
}

var InfiniteFuture = time.Unix(1<<63-62135596801, 999999999)

func (i *cacheItem[Value]) updateExpire(ttl time.Duration) {
	if ttl <= 0 {
		i.expire = InfiniteFuture
	} else {
		i.expire = time.Now().Add(ttl)
	}
}
