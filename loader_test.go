package loader

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConcurrencySingleKey(t *testing.T) {
	var counter int32
	fetch := func(ctx context.Context, key string) (string, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(100 * time.Millisecond)
		return key, nil
	}
	l := New(fetch, 500*time.Millisecond, WithErrorTTL(5*time.Second))
	type result struct {
		dur time.Duration
		val interface{}
	}
	c := make(chan *result, 3)

	var start time.Time
	var dur time.Duration

	start = time.Now()
	for i := 0; i < 3; i++ {
		go func() {
			start := time.Now()
			val, _ := l.Load("x")
			c <- &result{val: val, dur: time.Since(start)}
		}()
		time.Sleep(10 * time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		res := <-c
		assert.InDelta(t, 100, res.dur.Milliseconds(), 25, "each get should within 1s")
		assert.Equal(t, "x", res.val, "Value must be x")
	}
	dur = time.Since(start)
	assert.InDelta(t, 100, dur.Milliseconds(), 25, "all get should within 1s")

	start = time.Now()
	val, _ := l.Load("x")
	dur = time.Since(start)
	assert.Less(t, dur.Milliseconds(), int64(50), "After cached get must be fast")
	assert.Equal(t, "x", val, "Value must still be x")

	assert.Equal(t, int32(1), counter, "fetch must be called once")
}

func TestConcurrencyMultiKey(t *testing.T) {
	var counter int32
	fetch := func(ctx context.Context, key int) (string, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(100 * time.Millisecond)
		return fmt.Sprint(key), nil
	}
	l := New(fetch, 500*time.Millisecond)
	type result struct {
		dur time.Duration
		val string
	}
	c := make(chan *result, 3)

	var start time.Time
	var dur time.Duration

	start = time.Now()
	for i := 0; i < 3; i++ {
		go func(i int) {
			start := time.Now()
			val, _ := l.Load(i)
			c <- &result{val: val, dur: time.Since(start)}
		}(i)
		time.Sleep(10 * time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		res := <-c
		assert.InDelta(t, 100, res.dur.Milliseconds(), 25, "each get should within 1s")
		assert.Equal(t, fmt.Sprint(i), res.val, "Value must be valid")
	}
	dur = time.Since(start)
	assert.InDelta(t, 100, dur.Milliseconds(), 25, "all get should within 1s")

	start = time.Now()
	val, _ := l.Load(1)
	dur = time.Since(start)
	assert.Less(t, dur.Milliseconds(), int64(50), "After cached get must be fast")
	assert.Equal(t, "1", val, "Value must still the same")

	assert.Equal(t, int32(3), counter, "fetch must be called once")
}

func TestExpire(t *testing.T) {
	var counter int32
	fetch := func(ctx context.Context, key string) (string, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond)
		return fmt.Sprintf("%d %s", counter, key), nil
	}
	l := New(fetch, 500*time.Millisecond)
	val, _ := l.Load("x")
	assert.Equal(t, "1 x", val, "First call")
	assert.Equal(t, int32(1), counter, "fetch called once")

	time.Sleep(550 * time.Millisecond)
	val, _ = l.Load("x")
	assert.Equal(t, "1 x", val, "Use stale value")
	val, _ = l.Load("x")
	assert.Equal(t, "1 x", val, "Still use stale value")

	time.Sleep(100 * time.Millisecond)
	val, _ = l.Load("x")
	assert.Equal(t, "2 x", val, "Use updated value")
	assert.Equal(t, int32(2), counter, "fetch called twice")
}

func TestDoubleCheckLockLeak(t *testing.T) {
	// 1. Setup with LRU size 1 to allow easy eviction
	// We use a latch to coordinate the race exactly.
	fetchLatch := make(chan struct{})

	fetch := func(ctx context.Context, key string) (string, error) {
		// Wait for signal to complete fetch
		// This allows us to keep T1 holding the lock while T2 blocks
		if key == "race_key" {
			select {
			case <-fetchLatch:
			case <-time.After(1 * time.Second):
			}
		}
		return "value", nil
	}

	// Create loader with LRU size 1
	// Note: We need to use WithLRU.
	// But WithLRU creates a driver. We need to make sure we are testing the Loader logic.
	l := New(fetch, 1*time.Hour, WithLRU(1))

	var wg sync.WaitGroup
	wg.Add(2)

	// 2. Trigger the Race

	// T1: The "Filler"
	go func() {
		defer wg.Done()
		l.Load("race_key")
	}()

	// T2: The "Victim"
	go func() {
		defer wg.Done()
		// Wait a tiny bit to ensure T1 started and acquired the lock
		time.Sleep(10 * time.Millisecond)

		// This should block on T1's lock.
		// When T1 finishes (signal sent below), T1 puts item in cache and unlocks.
		// T2 wakes up, gets lock, sees item in cache (double check), and returns.
		// BUG: T2 does not unlock.
		l.Load("race_key")
	}()

	// Allow T1 to proceed and finish
	time.Sleep(50 * time.Millisecond)
	close(fetchLatch)

	wg.Wait()

	// At this point, "race_key" is in cache.
	// If bug exists, "race_key" lock is held by T2 (leaked).

	// 3. Verify
	// We must force `Load` to try locking again.
	// Currently `fastLoad` returns true because item is in cache.
	// We must Evict "race_key".
	// Since LRU size is 1, loading "other_key" should evict "race_key".
	l.Load("other_key")

	// Now "race_key" should be gone from driver.
	// Verify it's gone (optional, implicit in next step)

	done := make(chan struct{})
	go func() {
		// This should fail fastLoad (not in cache), and try to acquire Lock.
		// If leaked, it hangs.
		l.Load("race_key")
		close(done)
	}()

	select {
	case <-done:
		// Success! Lock was available.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Deadlock: failed to acquire lock after eviction. Lock leak confirmed.")
	}
}
