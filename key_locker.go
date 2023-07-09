package loader

import (
	"sync"
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
	item := l.getItem(key)
	item.m.Lock()

	unlocked := false
	return func() {
		if unlocked {
			return
		}
		item.m.Unlock()
		l.releaseItem(key)

		unlocked = true
	}
}

func (l *InMemoryKeyLocker[Key]) getItem(key Key) *inMemoryKeyLockerItem {
	l.root.Lock()
	defer l.root.Unlock()

	item, ok := l.locks[key]
	if !ok {
		item = &inMemoryKeyLockerItem{}
		l.locks[key] = item
	}

	item.ref += 1
	return item
}

func (l *InMemoryKeyLocker[Key]) releaseItem(key Key) {
	l.root.Lock()
	defer l.root.Unlock()

	item, ok := l.locks[key]
	if !ok {
		return
	}

	item.ref -= 1
	if item.ref <= 0 {
		delete(l.locks, key)
	}
}

var _ KeyLocker[int] = &InMemoryKeyLocker[int]{}
var _ KeyLocker[string] = &InMemoryKeyLocker[string]{}
