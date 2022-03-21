package loader

import (
	"sync"
	"sync/atomic"
)

type KeyLocker[Key comparable] interface {
	Lock(key Key) (unlock func())
}

func newInMemoryKeyLocker[Key comparable]() KeyLocker[Key] {
	return &InMemoryKeyLocker[Key]{
		locks: map[Key]*inMemoryKeyLockerItem{},
	}
}

type InMemoryKeyLocker[Key comparable] struct {
	root  sync.Mutex
	locks map[Key]*inMemoryKeyLockerItem
}

type inMemoryKeyLockerItem struct {
	ref int32
	m   sync.Mutex
}

// Lock implements KeyLocker
func (l *InMemoryKeyLocker[Key]) Lock(key Key) func() {
	l.root.Lock()
	defer l.root.Unlock()

	item, ok := l.locks[key]
	if !ok {
		item = &inMemoryKeyLockerItem{}
		l.locks[key] = item
	}

	atomic.AddInt32(&item.ref, 1)
	item.m.Lock()

	unlocked := false
	return func() {
		if unlocked {
			return
		}
		item.m.Unlock()
		if atomic.AddInt32(&item.ref, -1) <= 0 {
			l.root.Lock()
			defer l.root.Unlock()
			delete(l.locks, key)
		}
		unlocked = true
	}
}

var _ KeyLocker[int] = &InMemoryKeyLocker[int]{}
var _ KeyLocker[string] = &InMemoryKeyLocker[string]{}
