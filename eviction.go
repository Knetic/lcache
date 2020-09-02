package lcache

type evictableEntry struct {
	key   string
	value *cacheEntry
}

// Removes the least-recently-used entry.
func (this *cache) removeLRU() {

	var oldest evictableEntry
	var i uint32
	var removableIdx int

	// eviction methodology stolen from redis 2.8 http://antirez.com/news/109

	// we can't actually sample the set of keys at random, but we can iterate and randomly add entries.
	// fortunately, Go maps return unordered tuples when range'd. Even when no changes to maps are made.
	// this makes random sampling pretty easy.
	for k, v := range this.entries {
		sample := evictableEntry{
			key:   k,
			value: v,
		}

		// add sample to pool.
		if this.evictionPoolTop < len(this.evictionPool) {
			this.evictionPool[this.evictionPoolTop] = sample
			this.evictionPoolTop++
		} else {

			// pool's full. Find the oldest entry, replace. Otherwise discard.
			for z, entry := range this.evictionPool {
				if entry.value.lastUsed.After(sample.value.lastUsed) {
					this.evictionPool[z] = sample
					break
				}
			}

			// no entry is older than the sample. Discard sample.
		}

		i++
		if i >= this.Params.EvictionSampleSize {
			break
		}
	}

	// find oldest entry in pool
	for z, entry := range this.evictionPool {
		if entry.value.lastUsed.Before(oldest.value.lastUsed) {
			oldest = entry
			removableIdx = z
		}
	}

	// remove it from sample pool, make room at the end.
	this.evictionPool[removableIdx] = this.evictionPool[this.evictionPoolTop]
	this.evictionPoolTop--

	// delete from cache
	delete(this.entries, oldest.key)
}
