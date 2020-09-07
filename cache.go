package lcache

import (
	"time"
)

// Params describes how a Cache is configured.
type Params struct {
	Loader Loader

	ExpireAfterWrite time.Duration
	ExpireAfterRead  time.Duration

	MaximumEntries     uint32
	EvictionPoolSize   uint32
	EvictionSampleSize uint32
	NumShards          uint32

	GracefulRefresh bool
}

// Cache is an auto-populating LRU cache.
type Cache interface {
	Get(string) (interface{}, error)
	Set(string, interface{}) error
	RunRefresh() error
	StopRefresh()
}

// cache implements Cache
type cache struct {
	Params

	entries   ConcurrentMap
	refreshes chan string

	evictionPool    []evictableEntry
	evictionPoolTop int
}

// NewCache creates a new auto-populating LRU cache, configured as defined by the given Params.
func NewCache(params Params) (Cache, error) {
	var refreshes chan string

	if params.MaximumEntries <= 0 {
		params.MaximumEntries = 4096
	}

	// min 8; too small and it doesn't make any sense anyway.
	if params.EvictionPoolSize <= 8 {
		params.EvictionPoolSize = 8
	}

	// no sense in sampling more than we can fit in the pool
	if params.EvictionPoolSize < params.EvictionSampleSize {
		params.EvictionPoolSize = params.EvictionSampleSize
	}

	if params.NumShards < 16 {
		params.NumShards = 16
	}

	if params.GracefulRefresh {
		refreshes = make(chan string, 32)
	}

	ret := &cache{
		Params:       params,
		refreshes:    refreshes,
		entries:      NewConcurrentMap(params.NumShards),
		evictionPool: make([]evictableEntry, params.EvictionPoolSize),
	}

	if params.GracefulRefresh {
		go func() {
			_ = ret.RunRefresh()
		}()
	}

	return ret, nil
}

func (this *cache) Get(key string) (interface{}, error) {

	now := time.Now().UTC()

	// try to find a pre-existing entry
	entry, exists := this.entries.Get(key)
	if exists {

		// if we had an error last time refreshing, hand it back here, and delete the entry.
		if entry.refreshError != nil {
			this.entries.Del(key)
			return entry.value, entry.refreshError
		}

		// if the entry has expired, then it may be refreshed asynchronously
		if entry.expiration.Before(now) {

			if this.refreshes == nil {
				// refresh not available, remove.
				this.entries.Del(key)
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
	if this.Loader != nil {
		value, err := this.Loader.Load(key)
		if err != nil {
			// error during loading, hand it back.
			return 0, err
		}

		// insert.
		entry = &cacheEntry{value: value}
		entry.updateTimestamps(this.ExpireAfterWrite)

		// make sure we never go over the cache size.
		if uint32(this.entries.Len()) >= this.MaximumEntries {
			this.removeLRU()
		}

		this.entries.Set(key, entry)
		return value, nil
	}

	// not in cache & no loader
	return nil, nil
}

// Set populates the cache with the given key=val pair.
func (this *cache) Set(key string, val interface{}) error {
	now := time.Now().UTC()
	oldEntry, found := this.entries.Get(key)
	if found && oldEntry != nil {
		oldEntry.value = val
		oldEntry.expiration = now.Add(this.Params.ExpireAfterWrite)
		oldEntry.lastUsed = now
		oldEntry.refreshing = false
		oldEntry.refreshError = nil
		return nil
	}

	this.entries.Set(key, &cacheEntry{
		value:        val,
		expiration:   now.Add(this.Params.ExpireAfterWrite),
		lastUsed:     now,
		refreshing:   false,
		refreshError: nil,
	})

	// make sure we never go over the cache size.
	if uint32(this.entries.Len()) >= this.MaximumEntries {
		this.removeLRU()
	}
	return nil
}

/*
	goroutine.
	Waits for cache entries to be flagged as expired, and reloads them using the Loader.
	By default, every Cache starts one of these goroutines for themselves, only call again if you need multiple refreshers (it's safe).
*/
// RunRefresh refreshes entries flagged as expired, using the Loader, if configured.
func (this *cache) RunRefresh() error {
	if this.refreshes == nil {
		return refreshErrNotEnabled
	}

	if this.Loader == nil {
		return refreshErrNoLoader
	}

	for key := range this.refreshes {
		// ensure the entry even exists to be refreshed
		entry, found := this.entries.Get(key)
		if entry == nil {
			continue
		}
		if !found {
			entry.refreshError = refreshErrEntryNotFound
			continue
		}

		// load new value
		newValue, err := this.Loader.Load(key)
		if err != nil {
			entry.refreshError = err
			continue
		}

		// set the new value on entry
		entry.value = newValue
		entry.refreshing = false
		entry.refreshError = nil
	}

	return nil
}

// Stops all refreshing on this cache.
func (this *cache) StopRefresh() {
	close(this.refreshes)
}
