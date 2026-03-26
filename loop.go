package main

const ringSize = 32

// RingBuffer is a fixed-size ring buffer of uint64 IDs used to prevent
// clipboard sync loops. The zero value is ready to use.
type RingBuffer struct {
	buf   [ringSize]uint64
	pos   int
	count int
}

// Add inserts an ID into the ring buffer, advancing the write position.
func (r *RingBuffer) Add(id uint64) {
	r.buf[r.pos] = id
	r.pos = (r.pos + 1) % ringSize
	if r.count < ringSize {
		r.count++
	}
}

// Contains reports whether the given ID is present in the ring buffer.
func (r *RingBuffer) Contains(id uint64) bool {
	n := r.count
	if n > ringSize {
		n = ringSize
	}
	for i := 0; i < n; i++ {
		if r.buf[i] == id {
			return true
		}
	}
	return false
}
