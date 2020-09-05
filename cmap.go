package lcache

import "sync"
import "unsafe"

// KeyVal represents the key-value pair of a cache key and a cacheEntry
type KeyVal struct {
	key string
	val *cacheEntry
}

// ConcurrentMap provides map capabilities that are lock-free on reads and shard-locked on writes.
type ConcurrentMap interface {
	Get(key string) (*cacheEntry, bool)
	Set(key string, val *cacheEntry)
	Del(key string)
	Len() int
	All() chan *KeyVal
}

// cmap provides a concurrent map
type cmap struct {
	Shards []*Shard
}

// Shard represents a submap with a shared lock
type Shard struct {
	m    map[string]interface{}
	lock sync.RWMutex
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr

// NewConcurrentMap creates a new concurrent map
func NewConcurrentMap(numShards uint32) ConcurrentMap {
	shards := make([]*Shard, numShards)
	for i := range shards {
		shards[i] = &Shard{
			m:    make(map[string]interface{}),
			lock: sync.RWMutex{},
		}
	}
	return &cmap{
		Shards: shards,
	}
}

// Shard gets the shard for the given key.
func (m *cmap) Shard(key string) *Shard {
	return m.Shards[m.hash(key)]
}

func (m *cmap) hash(key string) int {
	hash := uint(memhash(unsafe.Pointer(&key), 0, uintptr(len(key))))
	// return int(hash % uint64(len(m.Shards)))
	return jumphash(hash, len(m.Shards))
}

// Get retrieves the value associated with the key, if it exists.
func (m *cmap) Get(key string) (*cacheEntry, bool) {
	shard := m.Shard(key)
	shard.lock.RLock()
	defer shard.lock.RUnlock()
	val, ok := shard.m[key]
	if val == nil {
		return nil, ok
	}
	return val.(*cacheEntry), ok
}

// Set sets the key-val pair.
func (m *cmap) Set(key string, val *cacheEntry) {
	shard := m.Shard(key)
	shard.lock.Lock()
	shard.m[key] = val
	shard.lock.Unlock()
}

// Del deletes the key.
func (m *cmap) Del(key string) {
	shard := m.Shard(key)
	shard.lock.Lock()
	delete(shard.m, key)
	shard.lock.Unlock()
}

// Len returns the length of the concurrent map. TODO: make this constnat-time
func (m *cmap) Len() int {
	val := 0
	for _, shard := range m.Shards {
		val += len(shard.m)
	}
	return val
}

func (m *cmap) All() chan *KeyVal {
	iter := make(chan *KeyVal)
	go func() {
		for _, shard := range m.Shards {
			for key, val := range shard.m {
				iter <- &KeyVal{key: key, val: val.(*cacheEntry)}
			}
		}
		close(iter)
	}()
	return iter
}

// ported from https://github.com/ceejbot/jumphash/blob/master/jump.cc#L6-L17
func jumphash(key uint, buckets int) int {
	var b, j int
	b = -1
	j = 0
	for j < buckets {
		b = j
		key = key*2862933555777941757 + 1
		j = (b + 1) * (1 << 31) / int(((key >> 33) + 1))
	}
	return b
}
