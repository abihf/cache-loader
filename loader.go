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

type keyLock struct {
	ref int
	sync.Mutex
}

// Loader manage items in cache and fetch them if not exist
type Loader struct {
	fn    LoadFunc
	cache Cache

	ttl    time.Duration
	errTtl time.Duration

	m  sync.Mutex
	kl map[interface{}]*keyLock
}

// New creates new Loader
func New(fn LoadFunc, ttl time.Duration, cache Cache) *Loader {
	return &Loader{
		fn:     fn,
		cache:  cache,
		ttl:    ttl,
		errTtl: 10 * time.Second,

		kl: map[interface{}]*keyLock{},
	}
}

// SetErrorTTL how long it takes for this Loader to cache error
// and refetch again
func (l *Loader) SetErrorTTL(ttl time.Duration) {
	l.errTtl = ttl
}

func (l *Loader) lockKey(key interface{}) {
	l.m.Lock()
	defer l.m.Unlock()

	locker, ok := l.kl[key]
	if !ok {
		locker = &keyLock{}
		l.kl[key] = locker
	}
	locker.ref++
	locker.Lock()
}

func (l *Loader) unlockKey(key interface{}) {
	l.m.Lock()
	defer l.m.Unlock()

	locker := l.kl[key]
	locker.ref--
	if locker.ref <= 0 {
		delete(l.kl, key)
	}
	locker.Unlock()
}

// Get the item.
// If it doesn't exist on cache, Loader will call LoadFunc once even when other go routine access the same key.
// If the item is expired, it will return old value while loading new one.
func (l *Loader) Get(key interface{}) (interface{}, error) {
	l.lockKey(key)
	var unlocked int32
	unlock := func() {
		if atomic.CompareAndSwapInt32(&unlocked, 0, 1) {
			l.unlockKey(key)
		}
	}
	defer unlock()

	cached, ok := l.cache.Get(key)
	if ok {
		unlock()

		item := cached.(*cacheItem)
		item.vm.RLock()
		defer item.vm.RUnlock()

		// if the item is expired and it's not doing refetch
		if item.shouldRefetch() {
			go l.refetch(key, item)
		}
		return item.value, item.err
	}

	item := &cacheItem{}
	item.vm.Lock()
	defer item.vm.Unlock()

	l.cache.Add(key, item)
	unlock()

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
	defer func() {
		item.rm.Lock()
		defer item.rm.Unlock()
		item.isFetching = false
	}()

	value, err := l.fn(key)

	item.vm.Lock()
	defer item.vm.Unlock()

	item.value, item.err = value, err
	if err != nil {
		item.updateExpire(l.errTtl)
	} else {
		item.updateExpire(l.ttl)
	}
}

type cacheItem struct {
	value      interface{}
	err        error
	expire     time.Time
	isFetching bool

	vm sync.RWMutex
	rm sync.Mutex
}

func (i *cacheItem) shouldRefetch() bool {
	i.rm.Lock()
	defer i.rm.Unlock()

	if !i.isFetching && i.expire.Before(time.Now()) {
		i.isFetching = true
		return true
	}
	return false
}

func (i *cacheItem) updateExpire(ttl time.Duration) {
	i.expire = time.Now().Add(ttl)
}
