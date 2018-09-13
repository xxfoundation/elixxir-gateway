////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	jww "github.com/spf13/jwalterweatherman"
	"runtime"
	"sync"
)

type stats struct {
	MemoryAllocated uint64
	NumThreads      int
}

var prevStats *stats
var statsMutex sync.Mutex

func PrintProfilingStatistics() {
	statsMutex.Lock()
	// Get Total Allocated Memory
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryAllocated := memStats.Alloc

	// Number of threads
	numThreads := runtime.NumGoroutine()

	curStats := &stats{
		MemoryAllocated: memoryAllocated,
		NumThreads:      numThreads,
	}

	memDelta := int64(memoryAllocated)
	threadDelta := numThreads

	if prevStats != nil {
		memDelta -= int64(prevStats.MemoryAllocated)
		threadDelta -= prevStats.NumThreads
	}

	prevStats = curStats

	plusOrMinus := "+"
	if memDelta < 0 {
		plusOrMinus = ""
	}
	jww.INFO.Printf("Total memory allocation: %d (%s%d)", memoryAllocated,
		plusOrMinus, memDelta)

	plusOrMinus = "+"
	if threadDelta < 0 {
		plusOrMinus = ""
	}
	jww.INFO.Printf("Total thread count: %d (%s%d)", numThreads,
		plusOrMinus, threadDelta)
	statsMutex.Unlock()
}
