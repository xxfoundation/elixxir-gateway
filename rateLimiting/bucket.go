package rateLimiting

import (
	"sync"
	"time"
)

type Bucket struct {
	capacity   uint      // maximum number of tokens the bucket can hold
	remaining  uint      // current number of tokens in the bucket
	leakRate   float64   // rate that the bucket leaks at [tokens/nanosecond]
	lastUpdate time.Time // time that the bucket was most recently updated
	mux        sync.Mutex
}

// This is an implementation of the leaky bucket algorithm:
// https://en.wikipedia.org/wiki/Leaky_bucket

// Generates a new empty bucket.
func Create(capacity uint, rate float64) *Bucket {
	return &Bucket{
		capacity:   capacity,
		remaining:  0,
		leakRate:   rate,
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

// Adds token to the bucket and updates the remaining number of tokens. Returns
// true if token was added; false if there was insufficient capacity to do so.
func (b *Bucket) Add(token uint) bool {
	b.mux.Lock()

	// Calculate the time elapsed since the last update, in nanoseconds
	elapsedTime := time.Now().Sub(b.lastUpdate).Nanoseconds()

	// Calculate the amount of tokens have leaked over the elapsed time
	r := uint(float64(elapsedTime) * b.leakRate)

	// Subtract the number of leaked tokens from the remaining tokens ensuring
	// that remaining is no less than zero.
	if r > b.remaining {
		b.remaining = 0
	} else {
		b.remaining -= r
	}

	// Add the token value to the bucket
	b.remaining += token

	b.lastUpdate = time.Now()

	if b.remaining > b.capacity {
		b.mux.Unlock()
		return false
	} else {
		b.mux.Unlock()
		return true
	}
}
