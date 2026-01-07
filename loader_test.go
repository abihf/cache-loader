package loader

import (
	"context"
	"fmt"
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
