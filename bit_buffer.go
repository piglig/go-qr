package go_qr

import (
	"fmt"
	"math"
)

// BitSet defines an interface that allows manipulation of a bitset.
type BitSet interface {
	getBit(i int) bool
	set(i int, value bool)
	len() int
}

type BitBuffer []bool

// len returns the length of the BitBuffer.
func (b *BitBuffer) len() int {
	return len(*b)
}

// set sets the bit at position i in the BitBuffer to value.
func (b *BitBuffer) set(i int, value bool) {
	if i >= len(*b) {
		b.grow(1 + i) // If index is beyond current length, grow buffer.
	}
	(*b)[i] = value
}

// getBit returns the bit at position i in the BitBuffer.
func (b *BitBuffer) getBit(i int) bool {
	if i >= len(*b) {
		return false
	}
	return (*b)[i]
}

// grow increases the size of the BitBuffer.
func (b *BitBuffer) grow(size int) {
	res := make(BitBuffer, size)
	copy(res, *b)
	*b = res
}

// appendBits appends val as a binary number of length bits to the end of the BitBuffer.
func (b *BitBuffer) appendBits(val, length int) error {
	if length < 0 || length > 31 || (val>>uint(length)) != 0 {
		return fmt.Errorf("value out of range")
	}
	if math.MaxInt32-b.len() < length {
		return fmt.Errorf("maximum length reached")
	}
	for i := length - 1; i >= 0; i-- {
		b.set(b.len(), getBit(val, i))
	}
	return nil
}

// appendData appends another BitBuffer to this BitBuffer.
func (b *BitBuffer) appendData(other *BitBuffer) error {
	if other == nil {
		return fmt.Errorf("BitBuffer is nil")
	}

	if math.MaxInt32-b.len() < other.len() {
		return fmt.Errorf("maximum length reached")
	}

	for i := 0; i < other.len(); i++ {
		bit := other.getBit(i)
		b.set(b.len(), bit)
	}
	return nil
}

// clone creates a copy of the BitBuffer.
func (b *BitBuffer) clone() *BitBuffer {
	clone := make(BitBuffer, len(*b))
	copy(clone, *b)
	return &clone
}
