package rateLimiting

import (
	"math"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	// Max time (in nanoseconds) between update and test
	lastUpdateWait := int64(1000)

	capacity := uint(10)
	rate := float64(0.000000000001389)
	b := Create(capacity, rate)

	if b.capacity != capacity {
		t.Errorf("Create() generated Bucket with incorrect capacity\n\treceived: %v\n\texpected: %v", b.capacity, capacity)
	}

	if b.remaining != 0 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 0)
	}

	if b.leakRate != rate {
		t.Errorf("Create() generated Bucket with incorrect rate\n\treceived: %v\n\texpected: %v", b.leakRate, rate)
	}

	if time.Now().Sub(b.lastUpdate).Nanoseconds() > lastUpdateWait {
		t.Errorf("Create() generated Bucket with incorrect lastUpdate or the time between creation and testing was greater than %v ns\n\treceived: %v\n\texpected: %v", lastUpdateWait, b.lastUpdate, time.Now())
	}

	capacity = 0
	rate = math.MaxFloat64
	b = Create(0, rate)

	if b.capacity != capacity {
		t.Errorf("Create() generated Bucket with incorrect capacity\n\treceived: %v\n\texpected: %v", b.capacity, capacity)
	}

	if b.remaining != capacity {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, capacity)
	}

	if b.leakRate != rate {
		t.Errorf("Create() generated Bucket with incorrect rate\n\treceived: %v\n\texpected: %v", b.leakRate, rate)
	}

	if time.Now().Sub(b.lastUpdate).Nanoseconds() > lastUpdateWait {
		t.Errorf("Create() generated Bucket with incorrect lastUpdate or the time between creation and testing was greater than %v ns\n\treceived: %v\n\texpected: %v", lastUpdateWait, b.lastUpdate, time.Now())
	}

	b = Create(math.MaxUint32, 0)

	if b.capacity != math.MaxUint32 {
		t.Errorf("Create() generated Bucket with incorrect capacity\n\treceived: %v\n\texpected: %v", b.capacity, math.MaxUint32)
	}

	if b.remaining != 0 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 0)
	}

	if b.leakRate != 0 {
		t.Errorf("Create() generated Bucket with incorrect rate\n\treceived: %v\n\texpected: %v", b.leakRate, 0)
	}

	if time.Now().Sub(b.lastUpdate).Nanoseconds() > lastUpdateWait {
		t.Errorf("Create() generated Bucket with incorrect lastUpdate or the time between creation and testing was greater than %v ns\n\treceived: %v\n\texpected: %v", lastUpdateWait, b.lastUpdate, time.Now())
	}
}

func TestCapacity(t *testing.T) {
	capacity := uint(10)
	b := Create(capacity, 0.000000000001389)

	if b.Capacity() != capacity {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), capacity)
	}

	capacity = math.MaxUint32
	b = Create(capacity, 1)

	if b.Capacity() != capacity {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), capacity)
	}

	capacity = 0
	b = Create(capacity, math.MaxFloat64)

	if b.Capacity() != capacity {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), capacity)
	}
}

func TestRemaining(t *testing.T) {
	capacity := uint(10)
	b := Create(capacity, 0.000000000001389)

	if b.Remaining() != 0 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), 0)
	}

	capacity = math.MaxUint32
	b = Create(capacity, 1)

	if b.Remaining() != 0 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), 0)
	}

	capacity = 0
	b = Create(capacity, math.MaxFloat64)

	if b.Remaining() != capacity {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), capacity)
	}
}

func TestAdd_UnderLeakRate(t *testing.T) {
	b := Create(10, 0.000000003) // 3 per second

	addReturnVal := b.Add(9)

	if addReturnVal != true {
		t.Errorf("Add() failed to add when adding under the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 9 {
		t.Errorf("Add() returned incorrect remaining when adding under the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 9)
	}

	time.Sleep(2 * time.Second)
	addReturnVal = b.Add(7)

	if addReturnVal != true {
		t.Errorf("Add() failed to add when adding under the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 10 {
		t.Errorf("Add() returned incorrect remaining when adding under the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 10)
	}

	time.Sleep(4 * time.Second)
	addReturnVal = b.Add(2)

	if addReturnVal != true {
		t.Errorf("Add() failed to add when adding under the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 2 {
		t.Errorf("Add() returned incorrect remaining when adding under the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 2)
	}

	addReturnVal = b.Add(6)

	if addReturnVal != true {
		t.Errorf("Add() failed to add when adding under the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 8 {
		t.Errorf("Add() returned incorrect remaining when adding under the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 8)
	}
}

func TestAdd_OverLeakRate(t *testing.T) {
	b := Create(10, 0.000000003) // 3 per second

	addReturnVal := b.Add(17)

	if addReturnVal != false {
		t.Errorf("Add() incorrectly added when adding over the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, false)
	}

	if b.remaining != 17 {
		t.Errorf("Add() returned incorrect remaining when adding over the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 17)
	}

	time.Sleep(6 * time.Second)
	addReturnVal = b.Add(712)

	if addReturnVal != false {
		t.Errorf("Add() incorrectly added when adding over the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, false)
	}

	if b.remaining != 712 {
		t.Errorf("Add() returned incorrect remaining when adding over the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 712)
	}

	addReturnVal = b.Add(85)

	if addReturnVal != false {
		t.Errorf("Add() incorrectly added when adding over the leak rate\n\treceived: %v\n\texpected: %v", addReturnVal, false)
	}

	if b.remaining != 797 {
		t.Errorf("Add() returned incorrect remaining when adding over the leak rate\n\treceived: %v\n\texpected: %v", b.remaining, 797)
	}
}

func TestAdd_ReturnToNormal(t *testing.T) {
	b := Create(10, 0.000000003) // 3 per second

	addReturnVal := b.Add(12)

	if addReturnVal != false {
		t.Errorf("Add() incorrectly added\n\treceived: %v\n\texpected: %v", addReturnVal, false)
	}

	if b.remaining != 12 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 12)
	}

	time.Sleep(2 * time.Second)
	addReturnVal = b.Add(2)

	if addReturnVal != true {
		t.Errorf("Add() failed to add\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 8 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 8)
	}

	time.Sleep(1 * time.Second)
	addReturnVal = b.Add(5)

	if addReturnVal != true {
		t.Errorf("Add() failed to add\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 10 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 10)
	}

	time.Sleep(3 * time.Second)

	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)
	addReturnVal = b.Add(1)

	if addReturnVal != true {
		t.Errorf("Add() failed to add\n\treceived: %v\n\texpected: %v", addReturnVal, true)
	}

	if b.remaining != 9 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 9)
	}

	addReturnVal = b.Add(7)

	if addReturnVal != false {
		t.Errorf("Add() failed to add\n\treceived: %v\n\texpected: %v", addReturnVal, false)
	}

	if b.remaining != 16 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 16)
	}

	time.Sleep(3 * time.Second)
	addReturnVal = b.Add(7)

	if addReturnVal != false {
		t.Errorf("Add() incorrectly added\n\treceived: %v\n\texpected: %v", addReturnVal, false)
	}

	if b.remaining != 14 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 14)
	}
}

func TestAddLock(t *testing.T) {
	b := Create(10, 0.000000001)

	result := make(chan bool)

	b.mux.Lock()

	go func() {
		b.Add(15)
		result <- true
	}()

	select {
	case <-result:
		t.Errorf("Add() did not correctly lock the thread")
	case <-time.After(5 * time.Second):
		return
	}
}
