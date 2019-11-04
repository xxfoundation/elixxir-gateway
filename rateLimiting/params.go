package rateLimiting

import "time"

type Params struct {
	// Leak rate for newly created IP address buckets
	IpLeakRate float64
	// Leak rate for newly created user ID buckets
	UserLeakRate float64
	// Capacity for newly created IP address buckets
	IpCapacity uint
	// Capacity for newly created user ID buckets
	UserCapacity uint
	// How often to look for and discard stale buckets
	CleanPeriod time.Duration
	// Age of stale buckets when discarded
	MaxDuration time.Duration
	// File path to IP address whitelist file
	IpWhitelistFile string
	// File path to user ID whitelist file
	UserWhitelistFile string
}
