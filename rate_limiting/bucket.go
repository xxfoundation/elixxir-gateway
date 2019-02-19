package rate_limiting

import (
	"sync"
	"time"
)

type Bucket struct {
	capacity   uint      // maximum number of tokens the bucket can hold
	remaining  uint      // current number of tokens in the bucket
	lastUpdate time.Time // last time the bucket was updated
	mux        sync.Mutex
}

// Generates a new empty bucket.
func Create(capacity uint) Bucket {
	return Bucket{capacity: capacity, remaining: capacity, lastUpdate: time.Now()}
}

// Returns the capacity of the bucket.
func (b *Bucket) Capacity() uint {
	return b.capacity
}

// Returns the remaining space in the bucket.
func (b *Bucket) Remaining() uint {
	return b.remaining
}

// Add token to the bucket. Returns true if token was added and false if there
// was insufficient capacity to do so.
func (b *Bucket) Add(token uint) bool {
	b.mux.Lock()

	if b.remaining == 0 {
		return false
	} else {
		b.remaining--
		return true
	}
}
