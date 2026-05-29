package go_qr

import (
	"fmt"
	"math"
)

// BitBuffer is an append-only sequence of bits, packed eight to a byte
// (most-significant bit first). It is used to assemble QR segment and codeword
// data before it is read back out into the module matrix. Packing into bytes
// (rather than one bool per bit) keeps it compact, and append-based growth keeps
// appends amortized O(1).
type BitBuffer struct {
	data []byte // packed bits; bit i lives at data[i/8], MSB-first
	n    int    // number of valid bits
}

// len returns the number of bits in the buffer.
func (b *BitBuffer) len() int { return b.n }

// getBit returns the i-th bit. Out-of-range indices read as 0.
func (b *BitBuffer) getBit(i int) bool {
	if i < 0 || i >= b.n {
		return false
	}
	return b.data[i>>3]&(0x80>>uint(i&7)) != 0
}

// appendBit appends a single bit to the end of the buffer.
func (b *BitBuffer) appendBit(set bool) {
	if b.n>>3 >= len(b.data) {
		b.data = append(b.data, 0)
	}
	if set {
		b.data[b.n>>3] |= 0x80 >> uint(b.n&7)
	}
	b.n++
}

// appendBits appends the low length bits of val, most-significant bit first.
func (b *BitBuffer) appendBits(val, length int) error {
	if length < 0 || length > 31 || val>>uint(length) != 0 {
		return fmt.Errorf("value out of range")
	}
	if math.MaxInt32-b.n < length {
		return fmt.Errorf("maximum length reached")
	}
	for i := length - 1; i >= 0; i-- {
		b.appendBit((val>>uint(i))&1 != 0)
	}
	return nil
}

// appendData appends every bit of other to this buffer.
func (b *BitBuffer) appendData(other *BitBuffer) error {
	if other == nil {
		return fmt.Errorf("BitBuffer is nil")
	}
	if math.MaxInt32-b.n < other.n {
		return fmt.Errorf("maximum length reached")
	}
	for i := 0; i < other.n; i++ {
		b.appendBit(other.getBit(i))
	}
	return nil
}

// clone returns a deep copy of the buffer.
func (b *BitBuffer) clone() *BitBuffer {
	if b.data == nil {
		return &BitBuffer{n: b.n}
	}
	d := make([]byte, len(b.data))
	copy(d, b.data)
	return &BitBuffer{data: d, n: b.n}
}
