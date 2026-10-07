package qr

// bitBuffer is an append-only sequence of bits, packed eight to a byte
// (most-significant bit first). The zero value is an empty buffer.
type bitBuffer struct {
	data []byte // packed bits; bit i lives at data[i/8], MSB-first
	n    int    // number of valid bits
}

// len returns the number of bits in the buffer.
func (b *bitBuffer) len() int { return b.n }

// grow ensures room for another n bits without reallocating.
func (b *bitBuffer) grow(n int) {
	need := (b.n + n + 7) / 8
	if need > cap(b.data) {
		d := make([]byte, len(b.data), need)
		copy(d, b.data)
		b.data = d
	}
}

// getBit returns the i-th bit. Out-of-range indices read as 0.
func (b *bitBuffer) getBit(i int) bool {
	if i < 0 || i >= b.n {
		return false
	}
	return b.data[i>>3]&(0x80>>uint(i&7)) != 0
}

// appendBit appends a single bit.
func (b *bitBuffer) appendBit(set bool) {
	if b.n>>3 >= len(b.data) {
		b.data = append(b.data, 0)
	}
	if set {
		b.data[b.n>>3] |= 0x80 >> uint(b.n&7)
	}
	b.n++
}

// appendBits appends the low length bits of val, most-significant bit first.
// Callers guarantee 0 <= length <= 31 and 0 <= val < 1<<length.
func (b *bitBuffer) appendBits(val, length int) {
	if length < 0 || length > 31 || val>>uint(length) != 0 {
		panic("qr: bitBuffer.appendBits value out of range")
	}
	for i := length - 1; i >= 0; i-- {
		b.appendBit((val>>uint(i))&1 != 0)
	}
}

// appendBuffer appends every bit of other.
func (b *bitBuffer) appendBuffer(other *bitBuffer) {
	b.grow(other.n)
	if b.n%8 == 0 {
		// Byte-aligned: copy whole bytes. Bits past other.n are zero.
		b.data = append(b.data[:b.n/8], other.data[:(other.n+7)/8]...)
		b.n += other.n
		return
	}
	for i := 0; i < other.n; i++ {
		b.appendBit(other.getBit(i))
	}
}

// bytes returns the buffer packed into bytes; the final byte is zero-padded.
func (b *bitBuffer) bytes() []byte {
	return b.data[:(b.n+7)/8]
}
