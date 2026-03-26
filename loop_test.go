package main

import "testing"

func TestRingBufferEmpty(t *testing.T) {
	var r RingBuffer
	if r.Contains(1) {
		t.Fatal("Contains returned true on empty buffer")
	}
}

func TestRingBufferAddContains(t *testing.T) {
	var r RingBuffer
	r.Add(42)
	if !r.Contains(42) {
		t.Fatal("Contains returned false for added ID")
	}
}

func TestRingBufferNotContains(t *testing.T) {
	var r RingBuffer
	r.Add(1)
	r.Add(2)
	r.Add(3)
	if r.Contains(99) {
		t.Fatal("Contains returned true for ID not added")
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	var r RingBuffer
	// Add more than ringSize entries so the oldest are evicted.
	for i := uint64(1); i <= ringSize+10; i++ {
		r.Add(i)
	}
	// The first 10 entries should have been evicted.
	for i := uint64(1); i <= 10; i++ {
		if r.Contains(i) {
			t.Fatalf("Contains returned true for evicted ID %d", i)
		}
	}
	// The newest entries should still be present.
	for i := uint64(11); i <= ringSize+10; i++ {
		if !r.Contains(i) {
			t.Fatalf("Contains returned false for ID %d that should be present", i)
		}
	}
}

func TestRingBufferFull(t *testing.T) {
	var r RingBuffer
	for i := uint64(0); i < ringSize; i++ {
		r.Add(i + 100)
	}
	for i := uint64(0); i < ringSize; i++ {
		if !r.Contains(i + 100) {
			t.Fatalf("Contains returned false for ID %d in full buffer", i+100)
		}
	}
}

func TestRingBufferZeroValue(t *testing.T) {
	var r RingBuffer
	// Zero value should work without any initialization.
	if r.Contains(0) {
		t.Fatal("zero-value buffer should not contain anything before Add")
	}
	r.Add(7)
	if !r.Contains(7) {
		t.Fatal("Contains returned false after Add on zero-value buffer")
	}
}
