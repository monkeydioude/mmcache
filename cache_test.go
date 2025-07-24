package mmcache

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestICanCacheStuffForAFixedAmountOfTime(t *testing.T) {
	type dummy struct{ word string }

	test1 := New[string, int](5 * time.Minute)
	t.Log("we should return the newly cached value")
	v1, err := test1.Gset("a", func() (int, error) { return 1, nil })
	CmpNoError(t, err)
	Cmp(t, v1, 1)
	// we should return cached value
	t.Log("we should return the previously cached value")
	v1, err = test1.Gset("a", func() (int, error) {
		t.Fatalf("cache function should not be called here")
		return 2, nil
	})
	CmpNoError(t, err)
	Cmp(t, v1, 1)
	t.Log("for the same key, we should not return the first cached value")
	test2 := New[string, dummy](5 * time.Millisecond)
	_, err = test2.Gset("b", func() (dummy, error) { return dummy{"aw hell naw"}, nil })
	CmpNoError(t, err)
	time.Sleep(6 * time.Millisecond)
	v2, err := test2.Gset("b", func() (dummy, error) { return dummy{"cabane123"}, nil })
	CmpNoError(t, err)
	Cmp(t, v2.word, "cabane123")
}

func TestSingleflightOnlyCallsMakerOnce(t *testing.T) {
	cache := New[string, int](100 * time.Millisecond)
	callCount := 0
	valueGoal := 11
	var mu sync.Mutex

	// we will simulate a kinda long, locked cache function
	maker := func() (int, error) {
		mu.Lock()
		// Should be incremented only once
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return valueGoal, nil
	}

	n := 33
	wg := sync.WaitGroup{}
	wg.Add(n)
	// we make an array storing the results of each goroutine calling our cache
	results := make([]int, n)
	// here we simulate several call to the same key and cache function
	// to make sure our implementation of singleflight works as it should.
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			// We should execute `maker` only once
			v, err := cache.Gset("a_key", maker)
			CmpNoError(t, err)
			results[idx] = v
		}(i)
	}
	wg.Wait()
	Cmp(t, callCount, 1)
	for _, v := range results {
		Cmp(t, v, valueGoal)
	}
}

func TestMakerErrorWithinCacheFunction(t *testing.T) {
	errMakerFail := errors.New("maker fail")
	cache := New[string, int](time.Minute)
	first := true
	maker := func() (int, error) {
		if first {
			first = false
			return 0, errMakerFail
		}
		return 7, nil
	}
	t.Log(`we should get an "errMakerFail" error`)
	_, err := cache.Gset("fail", maker)
	CmpError(t, err)
	if !errors.Is(err, errMakerFail) {
		t.Fatal("expected errMakerFail error, got nil")
	}
	t.Log("we should not get an error now, since first == false")
	v, err := cache.Gset("fail", maker)
	CmpNoError(t, err)
	Cmp(t, v, 7)
}

func TestWritingConcurrentlyOnDifferentKeys(t *testing.T) {
	cache := New[string, int](time.Hour)
	goal1 := 1
	key1 := "key_a"
	goal2 := 2
	key2 := "key_b"

	var count1 int32 = 0
	var count2 int32 = 0

	// declare 2 maker function, each used on 2 different keys.
	// Using the atomic package to increment correctly in a concurrent setting
	maker1 := func() (int, error) {
		if atomic.AddInt32(&count1, 1) > 1 {
			t.Fatal("maker1 should not be called more than once")
		}
		return goal1, nil
	}
	maker2 := func() (int, error) {
		if atomic.AddInt32(&count2, 1) > 1 {
			t.Fatal("maker2 should not be called more than once")
		}
		return goal2, nil
	}

	n := 3333
	wg := sync.WaitGroup{}
	wg.Add(n)
	// we make an array storing the "got" and the "expected" of each goroutine calling our cache
	results := make([][2]int, n)
	// here we simulate several goroutines calling 2 different keys and cache functions.
	// we make sure concurrent calls to our cache
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			key := key1
			maker := maker1
			results[idx][1] = goal1
			if idx%2 == 0 {
				key = key2
				maker = maker2
				results[idx][1] = goal2
			}
			v, err := cache.Gset(key, maker)
			CmpNoError(t, err)
			results[idx][0] = v
		}(i)
	}
	wg.Wait()

	for _, v := range results {
		Cmp(t, v[0], v[1])
	}
	Cmp(t, atomic.LoadInt32(&count1), int32(1))
	Cmp(t, atomic.LoadInt32(&count2), int32(1))
}

func TestConcurrentDelete(t *testing.T) {
	cache := New[string, int](time.Hour)
	n := 33
	maxIt := n * 10
	keyPrefix := "key_"
	// Pre-populate with 100 keys
	for i := range maxIt {
		_, err := cache.Gset(fmt.Sprintf("%s%d", keyPrefix, i), func() (int, error) { return i, nil })
		CmpNoError(t, err)
	}

	var wg sync.WaitGroup
	wg.Add(n)
	// Each goroutine deletes a different chunk of keys
	for g := range n {
		go func(gid int) {
			defer wg.Done()
			var keys []string
			for i := gid * 10; i < (gid+1)*10; i++ {
				keys = append(keys, fmt.Sprintf("%s%d", keyPrefix, i))
			}
			cache.Delete(keys...)
		}(g)
	}
	wg.Wait()
	// Check that all keys are gone
	for i := range maxIt {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		// here we make sure we set and use the new maker, not the other ones
		// that should be deleted. We set a 1 hour cache duration, so if we
		// get the new maker's value, we can be sure the older ones got deleted.
		val, err := cache.Gset(key, func() (int, error) {
			return 123, nil
		})
		CmpNoError(t, err)
		Cmp(t, val, 123)
	}
}
