package lcache

import (
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"testing"
	"time"
)

var (
	key1   = "key1"
	key2   = "key2"
	key3   = "key3"
	keySet = []string{key1, key2, key3}
)

func TestGetStartWithCacheMiss(t *testing.T) {
	data := make(map[string]int, 0)
	params := Params{
		Loader:             &staticLoader{data: &data},
		MaximumEntries:     10,
		ExpireAfterWrite:   100 * time.Millisecond,
		ExpireAfterRead:    100 * time.Millisecond,
		EvictionPoolSize:   32,
		EvictionSampleSize: 32,
		GracefulRefresh:    true,
	}
	cache, _ := NewCache(params)

	// ensure nil values when cache and loader are blank
	ensureNotInCache(t, cache, key1)
	ensureNotInCache(t, cache, key2)
	ensureNotInCache(t, cache, key3)

	// loader is able to refresh key1
	data[key1] = 1

	// key1 hasn't expired yet so still results in negative cache hit
	ensureNotInCache(t, cache, key1)
	ensureNotInCache(t, cache, key2)
	ensureNotInCache(t, cache, key3)

	// value is refreshed (eventually)
	timeout := 2 * params.ExpireAfterRead
	ensureInCache(t, cache, key1, 1, timeout)
	ensureNotInCache(t, cache, key2)
	ensureNotInCache(t, cache, key3)

	// value is cached and does not need loader before expiry
	delete(data, key1)
	ensureInCache(t, cache, key1, 1, timeout)
	ensureNotInCache(t, cache, key2)
	ensureNotInCache(t, cache, key3)
}

func TestGetSetRaceCondition(t *testing.T) {
	data := make(map[string]int, len(keySet))
	params := Params{
		Loader:             &staticLoader{data: &data},
		MaximumEntries:     10,
		ExpireAfterWrite:   1 * time.Microsecond,
		ExpireAfterRead:    1 * time.Microsecond,
		EvictionPoolSize:   32,
		EvictionSampleSize: 32,
		GracefulRefresh:    false, // rule out refresh goroutine
	}
	cache, _ := NewCache(params)

	// spam Get
	go func() {
		for {
			for _, key := range keySet {
				_, err := cache.Get(key)
				if err != nil {
					t.Error(err.Error())
				}
			}
		}
	}()

	// spam Set
	go func() {
		for {
			for _, key := range keySet {
				err := cache.Set(key, rand.Intn(10))
				if err != nil {
					t.Error(err.Error())
				}
			}
		}
	}()

	time.Sleep(1 * time.Second)
}

func ensureNotInCache(t *testing.T, cache Cache, key string) {
	val, err := cache.Get(key)
	requireNoError(t, err)
	requireEqual(t, nil, val)
}

func ensureInCache(t *testing.T, cache Cache, key string, val interface{}, timeout time.Duration) {
	// if timeout == 0, don't deal with eventual consistency and just fetch from cache
	if timeout > 0 {
		past := time.Now()
		for {
			val, err := cache.Get(key1)
			if err != nil {
				log.Fatal("error fetching", err)
			}
			if val == 1 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		elapsed := time.Now().Sub(past)
		if elapsed > timeout {
			log.Fatal(fmt.Sprintf("eky was refreshed in %v but ExpireAfterRead=%v", elapsed, timeout))
		}
	}

	actual, err := cache.Get(key)
	requireNoError(t, err)
	requireEqual(t, actual, val, "unexpected cached value")
}

type staticLoader struct {
	data *map[string]int
}

func (loader *staticLoader) Load(key string) (interface{}, error) {
	if num, exists := (*loader.data)[key]; !exists {
		return nil, nil
	} else {
		return num, nil
	}
}

func requireEqual(t *testing.T, actual interface{}, expected interface{}, msgs ...string) {
	if actual != expected {
		t.Error(fmt.Sprintf("%s\nexpected:%v\nactual:%v\n", safeSprint(msgs), expected, actual))
		t.Log(string(debug.Stack()))
	}
}

func requireNoError(t *testing.T, err error, msgs ...string) {
	if err != nil {
		t.Error(fmt.Sprintf("%s\nexpected no error but got %s", safeSprint(msgs), err.Error()))
		t.Log(string(debug.Stack()))
	}
}

func safeSprint(msgs []string) string {
	if len(msgs) == 0 {
		return ""
	}
	return fmt.Sprint(msgs)
}
