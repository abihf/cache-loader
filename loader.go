package loader

import (
	"sync"
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
	ttl   time.Duration

	mutex sync.Mutex
}

// New creates new Loader
func New(fn LoadFunc, ttl time.Duration, cache Cache) *Loader {
	return &Loader{
		fn:    fn,
		cache: cache,
		ttl:   ttl,

		mutex: sync.Mutex{},
	}
}

// Get the item.
// If it doesn't exist on cache, Loader will call LoadFunc once even when other go routine access the same key.
// If the item is expired, it will return old value while loading new one.
func (l *Loader) Get(key interface{}) (interface{}, error) {
	l.mutex.Lock()
	cached, ok := l.cache.Get(key)
	if ok {
		defer l.mutex.Unlock()

		item := cached.(*cacheItem)
		item.mutex.Lock()
		defer item.mutex.Unlock()

		if item.expire.Before(time.Now()) && !item.isFetching {
			item.isFetching = true // so other thread don't fetch
			go l.refetch(key, item)
		}
		return item.value, nil
	}

	item := &cacheItem{isFetching: true, mutex: sync.Mutex{}}
	item.mutex.Lock()
	defer item.mutex.Unlock()
	defer func() {
		item.isFetching = false
	}()
	l.cache.Add(key, item)
	l.mutex.Unlock()

	value, err := l.fn(key)
	if err != nil {
		l.cache.Remove(key)
		return nil, err
	}
	item.value = value
	item.updateExpire(l.ttl)
	return value, nil
}

func (l *Loader) refetch(key interface{}, item *cacheItem) {
	item.isFetching = true // to make sure, lol
	defer func() {
		item.isFetching = false
	}()

	value, err := l.fn(key)
	if err != nil {
		l.cache.Remove(key)
		return
	}
	item.value = value
	item.updateExpire(l.ttl)
}

type cacheItem struct {
	value  interface{}
	expire time.Time

	mutex      sync.Mutex
	isFetching bool
}

func (i *cacheItem) updateExpire(ttl time.Duration) {
	newExpire := time.Now().Add(ttl)
	i.expire = newExpire
}
