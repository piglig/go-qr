package go_qr

import (
	"fmt"
	"math"
)

type BitSet interface {
	getBit(i int) bool
	set(i int, value bool)
	len() int
}

type BitBuffer []bool

func (b *BitBuffer) len() int {
	return len(*b)
}

func (b *BitBuffer) set(i int, value bool) {
	if i >= len(*b) {
		b.grow(1 + i)
	}
	(*b)[i] = value
}

func (b *BitBuffer) getBit(i int) bool {
	if i >= len(*b) {
		return false
	}
	return (*b)[i]
}

func (b *BitBuffer) grow(size int) {
	res := make(BitBuffer, size)
	copy(res, *b)
	*b = res
}

func (b *BitBuffer) appendBits(val, length int) error {
	if length < 0 || length > 31 || (val>>uint(length)) != 0 {
		return fmt.Errorf("value out of range")
	}
	if math.MaxInt-b.len() < length {
		return fmt.Errorf("maximum length reached")
	}
	for i := length - 1; i >= 0; i-- {
		b.set(b.len(), GetBit(val, i))
	}
	return nil
}

func (b *BitBuffer) appendData(other *BitBuffer) error {
	if other == nil {
		return fmt.Errorf("BitBuffer is nil")
	}

	if math.MaxInt-b.len() < other.len() {
		return fmt.Errorf("maximum length reached")
	}

	for i := 0; i < other.len(); i++ {
		bit := other.getBit(i)
		b.set(b.len(), bit)
	}
	return nil
}

func (b *BitBuffer) clone() *BitBuffer {
	clone := make(BitBuffer, len(*b))
	copy(clone, *b)
	return &clone
}
