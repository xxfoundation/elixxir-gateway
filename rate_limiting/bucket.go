package rate_limiting

import (
	"sync"
	"time"
)

type Bucket struct {
	capacity   int64     // maximum number of tokens the bucket can hold
	remaining  int64     // current number of tokens in the bucket
	rate       int64     // rate that the bucket leaks at
	lastUpdate time.Time // last time the bucket was updated
	mux        sync.Mutex
}

// Generates a new empty bucket.
func Create(capacity int64, rate int64) Bucket {
	return Bucket{
		capacity:   capacity,
		remaining:  capacity,
		rate:       rate,
		lastUpdate: time.Now(),
	}
}

// Returns the capacity of the bucket.
func (b *Bucket) Capacity() int64 {
	return b.capacity
}

// Returns the remaining space in the bucket.
func (b *Bucket) Remaining() int64 {
	return b.remaining
}

// Add token to the bucket and updates the remaining number of tokens. Returns
// true if token was added; false if there was insufficient capacity to do so.
func (b *Bucket) Add(token uint) bool {
	b.mux.Lock()

	// Update the amount remaining by calculating the number of leaks
	b.remaining -= time.Now().Sub(b.lastUpdate).Nanoseconds() * b.rate

	// Ensure remaining is no less than zero
	if b.remaining < 0 {
		b.remaining = 0
	}

	b.lastUpdate = time.Now()

	if b.remaining >= b.capacity {
		b.mux.Unlock()
		return false
	} else {
		b.mux.Unlock()
		return true
	}
}
