package rate_limiting

import (
	"sync"
	"time"
)

type Bucket struct {
	capacity   uint      // maximum number of tokens the bucket can hold
	remaining  uint      // current number of tokens in the bucket
	rate       float64   // rate that the bucket leaks at [leaks/nanosecond]
	lastUpdate time.Time // last time the bucket was updated
	mux        sync.Mutex
}

// Generates a new empty bucket.
func Create(capacity uint, rate float64) Bucket {
	return Bucket{
		capacity:   capacity,
		remaining:  capacity,
		rate:       rate,
		lastUpdate: time.Now(),
	}
}

// Returns the capacity of the bucket.
func (b *Bucket) Capacity() uint {
	return b.capacity
}

// Returns the remaining space in the bucket.
func (b *Bucket) Remaining() uint {
	return b.remaining
}

// Add token to the bucket and updates the remaining number of tokens. Returns
// true if token was added; false if there was insufficient capacity to do so.
func (b *Bucket) Add(token uint) bool {
	b.mux.Lock()

	// Calculate the time elapsed since the last update, in nanoseconds
	elapsedTime := time.Now().Sub(b.lastUpdate).Nanoseconds()

	// Calculate the amount of tokens have leaked over the elapsed time
	r := uint(float64(elapsedTime) * b.rate)

	// Update the remaining number tokens and ensure remaining is no less than zero
	if r > b.remaining {
		b.remaining = 0
	} else {
		b.remaining -= r
	}

	b.lastUpdate = time.Now()

	if b.remaining >= b.capacity {
		b.mux.Unlock()
		return false
	} else {
		b.remaining++
		b.mux.Unlock()
		return true
	}
}
