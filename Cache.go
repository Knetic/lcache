package lcache

import (
	"errors"
	"time"
)

type Params struct {
	Loader Loader

	MaximumEntries uint32
	ExpireAfterWrite time.Duration
	ExpireAfterRead time.Duration

	GracefulRefresh bool
}

type Cache struct {
	
	Params

	entries map[string]*cacheEntry
	refreshes chan string
}

func NewCache(params Params) (*Cache, error) {

	var refreshes chan string

	if params.MaximumEntries <= 0 {
		params.MaximumEntries = 4096
	}

	if params.GracefulRefresh {
		refreshes = make(chan string, 32)
	}

	ret := &Cache {
		Params: params,
		refreshes: refreshes,
	}
	
	if params.GracefulRefresh {
		go RunRefresh(ret)
	}

	return ret, nil
}

func (this *Cache) Get(key string) (interface{}, error) {

	now := time.Now()

	// try to find a pre-existing entry
	entry, exists := this.entries[key]
	if exists {

		// if we had an error last time refreshing, hand it back here, and delete the entry.
		if entry.refreshError != nil {
			delete(this.entries, key)
			return entry.value, entry.refreshError
		}

		if entry.expiration.After(now) {
			
			if this.refreshes == nil {
				// refresh not available, remove.
				delete(this.entries, key)
			} else {
				// if it's not already refreshing, queue it for refresh and prevent further queueing.
				if !entry.refreshing {
					entry.refreshing = true
					this.refreshes <- key
				}
			}
		}

		// always return an entry from cache, even if we expired it by doing so.
		return entry.value, entry.refreshError
	}

	// not found in cache, load.
	value, err := this.Loader.Load(key)
	if err != nil {
		// error during loading, hand it back.
		return 0, err
	}

	// insert.
	entry = &cacheEntry{value: value}
	entry.updateTimestamps(this.ExpireAfterWrite)

	// make sure we never go over the cache size.
	if uint32(len(this.entries)) >= this.MaximumEntries {
		this.removeLRU()
	}

	this.entries[key] = entry
	return value, nil
}

/*
	goroutine.
	Waits for cache entries to be flagged as expired, and reloads them using the Loader.
	By default, every Cache starts one of these goroutines for themselves, only call again if you need multiple refreshers (it's safe).
*/
func RunRefresh(cache *Cache) error {
	
	var entry *cacheEntry

	if cache.refreshes == nil {
		return errors.New("Cache was not created with graceful refresh enabled")
	}

	for key := range cache.refreshes {
		
		value, err := cache.Get(key)
		if err != nil {
			entry.refreshError = err
			continue
		}

		entry.value = value
		entry.refreshing = false
		entry.refreshError = nil
	}

	return nil
}

// Removes the least-recently-used entry.
func (this *Cache) removeLRU() {

	var lruKey string
	var lastUsed time.Time

	for k, v := range this.entries {
		if lastUsed.After(v.lastUsed) {
			lastUsed = v.lastUsed
			lruKey = k
		}
	}

	delete(this.entries, lruKey)
}

// Stops all refreshing on this cache.
func (this *Cache) StopRefresh() {
	close(this.refreshes)
}