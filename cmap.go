package lcache

import (
	"sync"
	"unsafe"
)

// KeyVal represents the key-value pair of a cache key and a cacheEntry
type KeyVal struct {
	Key string
	Val *cacheEntry
}

// ConcurrentMap provides map capabilities that are lock-free on reads and shard-locked on writes.
type ConcurrentMap interface {
	Get(key string) (*cacheEntry, bool)
	Set(key string, val *cacheEntry) (*cacheEntry, bool)
	Del(key string) (*cacheEntry, bool)
	Len() int
	All() chan *KeyVal
}

// cmap provides a concurrent map.
type cmap struct {
	Shards      []*Shard
	CountChange chan int
	Count       int
}

// Shard represents a submap with a shared lock
type Shard struct {
	m    map[string]interface{}
	lock sync.RWMutex
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr

// NewConcurrentMap creates a new concurrent map.
func NewConcurrentMap(numShards uint32) ConcurrentMap {
	countChangeChan := make(chan int, 32)
	shards := make([]*Shard, numShards)
	for i := range shards {
		shards[i] = &Shard{
			m:    make(map[string]interface{}),
			lock: sync.RWMutex{},
		}
	}
	ret := &cmap{
		Shards:      shards,
		CountChange: countChangeChan,
	}
	go ret.updateCount()
	return ret
}

// Shard gets the shard for the given key.
func (m *cmap) Shard(key string) *Shard {
	return m.Shards[m.hash(key)]
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
func (m *cmap) Set(key string, val *cacheEntry) (*cacheEntry, bool) {
	shard := m.Shard(key)
	shard.lock.Lock()
	oldVal, exists := shard.m[key]
	shard.m[key] = val
	shard.lock.Unlock()
	if !exists {
		m.CountChange <- 1
	}
	if oldVal == nil {
		return nil, exists
	}
	return oldVal.(*cacheEntry), exists
}

// Del deletes the key.
func (m *cmap) Del(key string) (*cacheEntry, bool) {
	shard := m.Shard(key)
	shard.lock.Lock()
	oldVal, exists := shard.m[key]
	delete(shard.m, key)
	shard.lock.Unlock()
	if exists {
		m.CountChange <- -1
	}
	if oldVal == nil {
		return nil, exists
	}
	return oldVal.(*cacheEntry), exists
}

// Len returns the length of the concurrent map. TODO: make this constnat-time.
func (m *cmap) Len() int {
	return m.Count
}

// All returns a channel that sends all key-val pairs in the map.
func (m *cmap) All() chan *KeyVal {
	iter := make(chan *KeyVal)
	go func() {
		count := 0
		for _, shard := range m.Shards {
			// can't send to channel while holding RLock, so buffer the key-value pairs
			shard.lock.RLock()
			keyVals := make([]*KeyVal, 0, len(shard.m))
			for key, val := range shard.m {
				keyVals = append(keyVals, &KeyVal{Key: key, Val: val.(*cacheEntry)})
			}
			shard.lock.RUnlock()

			for _, keyVal := range keyVals {
				count++
				iter <- keyVal
			}
		}
		println("All(): count:", count)
		close(iter)
	}()
	return iter
}

// updateCount is meant to be a goroutine and the only one that updates cmap.Count.
// This makes cmap.Count eventually consistent (when the channel has been drained).
func (m *cmap) updateCount() {
	for delta := range m.CountChange {
		m.Count += delta
	}
}

// uses memhash for fast hash (not consistent across processes)
// and jumphash for finding shard index
func (m *cmap) hash(key string) int {
	var hash uint64
	keyLen := len(key)
	if keyLen <= 16 {
		hash = uint64(memhash(unsafe.Pointer(&key), 0, uintptr(uint64(keyLen))))
	} else {
		start := 0
		for start < keyLen {
			end := minInt(start+16, keyLen)
			chunk := key[start:end]
			hash ^= uint64(memhash(unsafe.Pointer(&chunk), 0, uintptr(uint64(len(chunk)))))
			start = end
			if start == keyLen {
				break
			}
		}
	}
	bucket := jumphash(hash, len(m.Shards))
	return bucket
}

func minInt(n, m int) int {
	if n < m {
		return n
	}
	return m
}

// ported from https://github.com/ceejbot/jumphash/blob/master/jump.cc#L6-L17
func jumphash(key uint64, shards int) int {
	var b, j int
	b = -1
	j = 0
	for j < shards {
		b = j
		key = key*2862933555777941757 + 1
		j = (b + 1) * (1 << 31) / int(((key >> 33) + 1))
	}
	return b
}
