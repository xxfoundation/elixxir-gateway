package rate_limiting

import (
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
}

func TestCapacity(t *testing.T) {
	b := Create(10, 0.000000000001389)

	if b.Capacity() != 10 {
		t.Errorf("Capacity() returned incorrect capacity\n\treceived: %v\n\texpected: %v", b.Capacity(), 10)
	}
}

func TestRemaining(t *testing.T) {
	b := Create(10, 0.000000000001389)

	if b.Remaining() != 10 {
		t.Errorf("Remaining() returned incorrect remaining\n\treceived: %v\n\texpected: %v", b.Remaining(), 10)
	}
}

func TestAdd(t *testing.T) {
	b := Create(10, 0.000000000001389)

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
