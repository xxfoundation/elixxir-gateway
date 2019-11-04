package rateLimiting

import (
	"sync"
	"time"
)

type BucketMap struct {
	buckets      map[string]*Bucket // Map of the buckets
	newCapacity  uint               // The capacity of newly created buckets
	newLeakRate  float64            // The leak rate of newly created buckets
	cleanPeriod  time.Duration      // Duration between stale bucket removals
	maxDuration  time.Duration      // Max time of inactivity before removal
	sync.RWMutex                    // Only allows one writer at a time
}

// Creates and returns a new BucketMap interface.
func CreateBucketMap(newCapacity uint, newLeakRate float64,
	cleanPeriod, maxDuration time.Duration) BucketMap {
	bm := BucketMap{
		buckets:     make(map[string]*Bucket),
		newCapacity: newCapacity,
		newLeakRate: newLeakRate,
		cleanPeriod: cleanPeriod,
		maxDuration: maxDuration,
	}

	go bm.StaleBucketWorker()

	return bm
}

// Returns the bucket with the specified key. If no bucket exists, then a new
// one is created, inserted into the map, and returned.
func (bm *BucketMap) LookupBucket(key string) *Bucket {
	// Get the bucket and a boolean determining if it exists in the map
	bm.RLock()
	b, exists := bm.buckets[key]
	bm.RUnlock()

	// Check if the bucket exists
	if exists {
		// If the bucket exists, then return it
		return b
	} else {
		// If the bucket does not exist, lock the thread and check check again
		// to ensure no other changes are made

		bm.Lock()
		b, exists = bm.buckets[key]

		if !exists {
			// If the bucket does not exist, then create a new one
			// NOTE: I was unable to test that the key corresponds to the correct
			// bucket. If you end up putting actual values in here, you may
			// want to test for that.
			bm.buckets[key] = Create(bm.newCapacity, bm.newLeakRate)
			b = bm.buckets[key]
		}

		bm.Unlock()
		return b
	}
}

// Periodically clears stale buckets from the map.
func (bm *BucketMap) StaleBucketWorker() {
	c := time.Tick(bm.cleanPeriod)

	for range c {
		bm.clearStaleBuckets()
	}
}

// Remove stale buckets from the map. Stale buckets are buckets that have not
// been updated for the specified duration.
func (bm *BucketMap) clearStaleBuckets() {
	bm.RLock()

	// Loop through each bucket in the map
	for key, bucket := range bm.buckets {
		// Calculate time since the bucket's last update
		duration := time.Since(bucket.lastUpdate)

		// Remove from map if the bucket has not been updated for a while
		if duration >= bm.maxDuration {
			bm.RUnlock()
			bm.Lock()
			delete(bm.buckets, key)
			bm.Unlock()
			bm.RLock()
		}
	}

	bm.RUnlock()
}
