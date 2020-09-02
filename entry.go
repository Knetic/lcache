package lcache

import (
	"time"
)

type cacheEntry struct {
	value interface{}

	expiration time.Time
	lastUsed   time.Time

	refreshing   bool
	refreshError error
}

func (this *cacheEntry) updateTimestamps(expireAfter time.Duration) {
	this.lastUsed = time.Now()
	if expireAfter > 0 {
		this.expiration = this.lastUsed.Add(expireAfter)
	}
}
