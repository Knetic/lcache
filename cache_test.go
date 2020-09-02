package lcache

import (
	"fmt"
	"runtime/debug"
	"testing"
	"time"
)

var (
	key1 = "key1"
	key2 = "key2"
	key3 = "key3"
)

func TestGet(t *testing.T) {
	data := make(map[string]int, 0)
	params := Params{
		Loader:             &staticLoader{data: data},
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

	// sleep both durations to not care about tie-breakers: we just want the entry expired
	time.Sleep(params.ExpireAfterRead)
	time.Sleep(params.ExpireAfterWrite)

	// value is refreshed (eventually)
	ensureInCache(t, cache, key1, 1)
	ensureNotInCache(t, cache, key2)
	ensureNotInCache(t, cache, key3)

	// value is cached and does not need loader before expiry
	delete(data, key1)
	ensureInCache(t, cache, key1, 1)
	ensureNotInCache(t, cache, key2)
	ensureNotInCache(t, cache, key3)
}

func ensureNotInCache(t *testing.T, cache Cache, key string) {
	val, err := cache.Get(key)
	requireNoError(t, err)
	requireEqual(t, nil, val)
}

func ensureInCache(t *testing.T, cache Cache, key string, val interface{}) {
	actual, err := cache.Get(key)
	requireNoError(t, err)
	requireEqual(t, actual, val, "unexpected cached value")
}

type staticLoader struct {
	data map[string]int
}

func (loader *staticLoader) Load(key string) (interface{}, error) {
	if num, exists := loader.data[key]; !exists {
		println(fmt.Sprintf("nothing to load for [%s]", key))
		return nil, nil
	} else {
		println(fmt.Sprintf("loading key=%s val=%d\n", key, num))
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
