package go_qr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// bufFromBits builds a BitBuffer from a sequence of bits.
func bufFromBits(bits ...bool) *BitBuffer {
	b := &BitBuffer{}
	for _, bit := range bits {
		b.appendBit(bit)
	}
	return b
}

// bitsOf reads a BitBuffer back into a []bool for comparison.
func bitsOf(b *BitBuffer) []bool {
	out := make([]bool, b.len())
	for i := range out {
		out[i] = b.getBit(i)
	}
	return out
}

func TestBitBuffer_AppendBitAndGet(t *testing.T) {
	b := &BitBuffer{}
	if b.getBit(0) {
		t.Fatal("empty buffer should read 0 out of range")
	}

	pattern := []bool{true, false, true, true, false, false, true, false, true}
	for _, bit := range pattern {
		b.appendBit(bit)
	}
	if b.len() != len(pattern) {
		t.Fatalf("len = %d, want %d", b.len(), len(pattern))
	}
	assert.Equal(t, pattern, bitsOf(b))
	if b.getBit(len(pattern)) {
		t.Fatal("read past end should be 0")
	}
}

func TestBitBuffer_Clone(t *testing.T) {
	a := bufFromBits(true, false, true, true, false, true, true)
	b := a.clone()
	assert.Equal(t, bitsOf(a), bitsOf(b))

	// Mutating the clone must not affect the original.
	b.appendBit(true)
	if a.len() == b.len() {
		t.Fatal("clone is not independent of the original")
	}
}

func TestBitBuffer_AppendBits(t *testing.T) {
	tests := []struct {
		name     string
		val      int
		length   int
		wantErr  bool
		wantBits []bool
	}{
		{name: "positive value", val: 5, length: 3, wantBits: []bool{true, false, true}},
		{name: "zero padding", val: 0, length: 4, wantBits: []bool{false, false, false, false}},
		{name: "negative value", val: -100, length: 5, wantErr: true},
		{name: "value too wide for length", val: 8, length: 3, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BitBuffer{}
			err := b.appendBits(tt.val, tt.length)
			if (err != nil) != tt.wantErr {
				t.Fatalf("appendBits() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			assert.Equal(t, tt.wantBits, bitsOf(b))
		})
	}
}

func TestBitBuffer_AppendData(t *testing.T) {
	cases := []struct {
		name     string
		a        *BitBuffer
		b        *BitBuffer
		wantErr  bool
		wantBits []bool
	}{
		{
			name:    "nil source is an error",
			a:       &BitBuffer{},
			b:       nil,
			wantErr: true,
		},
		{
			name:     "append into empty",
			a:        &BitBuffer{},
			b:        bufFromBits(true, true, false, true, false, true),
			wantBits: []bool{true, true, false, true, false, true},
		},
		{
			name:     "append empty is a no-op",
			a:        bufFromBits(true, true, false, true, false, false),
			b:        &BitBuffer{},
			wantBits: []bool{true, true, false, true, false, false},
		},
		{
			name:     "concatenation across a byte boundary",
			a:        bufFromBits(false, false, true, true, false, true, false, true, true, false, true),
			b:        bufFromBits(true, true, false, true, false, false),
			wantBits: []bool{false, false, true, true, false, true, false, true, true, false, true, true, true, false, true, false, false},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.a.appendData(tt.b)
			if (err != nil) != tt.wantErr {
				t.Fatalf("appendData() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			assert.Equal(t, tt.wantBits, bitsOf(tt.a))
		})
	}
}
