package lcache

import (
	"time"
)

type cacheEntry struct {
	
	value interface{}

	expiration time.Time
	lastUsed time.Time
	
	refreshing bool
	refreshError error
}

func (this *cacheEntry) updateTimestamps(expireAfterRead time.Time) {

	this.lastUsed = time.Now()
	if this.ExpireAfterRead > 0 {
		this.expiration = this.lastUsed + expireAfterRead
	}
}