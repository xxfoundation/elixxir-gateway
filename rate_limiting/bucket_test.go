package rate_limiting

import (
	"math"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	b := Create(10, 0.000000000001389)

	if b.capacity != 10 {
		t.Errorf("Create() generated Bucket with incorrect capacity\n\treceived: %v\n\texpected: %v", b.capacity, 10)
	}

	if b.remaining != 10 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 10)
	}

	if b.rate != 0.000000000001389 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 0.000000000001389)
	}

	if !time.Now().Equal(b.lastUpdate) {
		t.Errorf("Create() generated Bucket with incorrect lastUpdate\n\treceived: %v\n\texpected: %v", b.lastUpdate, time.Now())
	}

	b = Create(0, math.MaxFloat64)

	if b.capacity != 0 {
		t.Errorf("Create() generated Bucket with incorrect capacity\n\treceived: %v\n\texpected: %v", b.capacity, 0)
	}

	if b.remaining != 0 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 0)
	}

	if b.rate != math.MaxFloat64 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, math.MaxFloat64)
	}

	if !time.Now().Equal(b.lastUpdate) {
		t.Errorf("Create() generated Bucket with incorrect lastUpdate\n\treceived: %v\n\texpected: %v", b.lastUpdate, time.Now())
	}

	b = Create(math.MaxUint32, 0)

	if b.capacity != math.MaxUint32 {
		t.Errorf("Create() generated Bucket with incorrect capacity\n\treceived: %v\n\texpected: %v", b.capacity, math.MaxUint32)
	}

	if b.remaining != math.MaxUint32 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, math.MaxUint32)
	}

	if b.rate != 0 {
		t.Errorf("Create() generated Bucket with incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 0)
	}

	if !time.Now().Equal(b.lastUpdate) {
		t.Errorf("Create() generated Bucket with incorrect lastUpdate\n\treceived: %v\n\texpected: %v", b.lastUpdate, time.Now())
	}
}

func TestCapacity(t *testing.T) {
	b := Create(10, 0.000000000001389)

	if b.Capacity() != 10 {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), 10)
	}

	b = Create(math.MaxUint32, 1)

	if b.Capacity() != math.MaxUint32 {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), math.MaxUint32)
	}

	b = Create(0, math.MaxFloat64)

	if b.Capacity() != 0 {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), 0)
	}
}

func TestRemaining(t *testing.T) {
	b := Create(10, 0.000000000001389)

	if b.Remaining() != 10 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), 10)
	}

	b = Create(math.MaxUint32, 1)

	if b.Remaining() != math.MaxUint32 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), math.MaxUint32)
	}

	b = Create(0, math.MaxFloat64)

	if b.Remaining() != 0 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), 0)
	}
}

func TestAdd(t *testing.T) {
	b := Create(10, 0.000000001)

	addBool := b.Add(7)

	if addBool != false {
		t.Errorf("Add() returned incorrect bool\n\treceived: %v\n\texpected: %v", addBool, false)
	}

	if b.remaining != 10 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 10)
	}

	time.Sleep(3 * time.Second)
	addBool = b.Add(15)

	if addBool != true {
		t.Errorf("Add() returned incorrect bool\n\treceived: %v\n\texpected: %v", addBool, true)
	}

	if b.remaining != 8 {
		t.Errorf("Add() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.remaining, 8)
	}

	time.Sleep(8 * time.Second)
	b.Add(15)

	if b.Remaining() != 1 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), 1)
	}

	result := make(chan bool)

	b.mux.Lock()

	go func() {
		b.Add(15)
		result <- true
	}()

	select {
	case <-result:
		t.Errorf("Add() did not correctly lock the thread")
	case <-time.After(10 * time.Second):
		return
	}
}
