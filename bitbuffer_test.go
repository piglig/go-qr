package qr

import (
	"testing"
)

// bufFromBits builds a bitBuffer from a sequence of bits.
func bufFromBits(bits ...bool) bitBuffer {
	var b bitBuffer
	for _, bit := range bits {
		b.appendBit(bit)
	}
	return b
}

// bitsOf reads a bitBuffer back into a []bool for comparison.
func bitsOf(b *bitBuffer) []bool {
	out := make([]bool, b.len())
	for i := range out {
		out[i] = b.getBit(i)
	}
	return out
}

func TestBitBuffer_AppendBitAndGet(t *testing.T) {
	var b bitBuffer
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
	assertEqual(t, pattern, bitsOf(&b))
	if b.getBit(len(pattern)) {
		t.Fatal("read past end should be 0")
	}
}

func TestBitBuffer_AppendBits(t *testing.T) {
	var b bitBuffer
	b.appendBits(5, 3)
	b.appendBits(0, 4)
	b.appendBits(0x7FFFFFFF, 31)
	want := append([]bool{true, false, true, false, false, false, false}, make([]bool, 31)...)
	for i := 7; i < len(want); i++ {
		want[i] = true
	}
	assertEqual(t, want, bitsOf(&b))
}

func TestBitBuffer_AppendBitsPanicsOutOfRange(t *testing.T) {
	for _, tc := range []struct{ val, length int }{{-100, 5}, {8, 3}, {0, 32}, {0, -1}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("appendBits(%d, %d) did not panic", tc.val, tc.length)
				}
			}()
			var b bitBuffer
			b.appendBits(tc.val, tc.length)
		}()
	}
}

func TestBitBuffer_AppendBuffer(t *testing.T) {
	cases := []struct {
		name     string
		a, b     bitBuffer
		wantBits []bool
	}{
		{
			name:     "append into empty",
			b:        bufFromBits(true, true, false, true, false, true),
			wantBits: []bool{true, true, false, true, false, true},
		},
		{
			name:     "append empty is a no-op",
			a:        bufFromBits(true, true, false, true, false, false),
			wantBits: []bool{true, true, false, true, false, false},
		},
		{
			name:     "concatenation across a byte boundary",
			a:        bufFromBits(false, false, true, true, false, true, false, true, true, false, true),
			b:        bufFromBits(true, true, false, true, false, false),
			wantBits: []bool{false, false, true, true, false, true, false, true, true, false, true, true, true, false, true, false, false},
		},
		{
			name:     "byte-aligned fast path",
			a:        bufFromBits(true, false, true, false, true, false, true, false),
			b:        bufFromBits(false, true, true, true, false, false, false, false, true, true),
			wantBits: []bool{true, false, true, false, true, false, true, false, false, true, true, true, false, false, false, false, true, true},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tt.a.appendBuffer(&tt.b)
			assertEqual(t, tt.wantBits, bitsOf(&tt.a))
			// Bits appended after a fast-path copy must land correctly.
			tt.a.appendBit(true)
			if !tt.a.getBit(tt.a.len() - 1) {
				t.Error("bit appended after appendBuffer is lost")
			}
		})
	}
}

func TestBitBuffer_AppendBufferDoesNotAlias(t *testing.T) {
	src := bufFromBits(true, false, true, true, false, true, true, false)
	var a, b bitBuffer
	a.appendBuffer(&src)
	b.appendBuffer(&src)
	a.appendBits(0xFF, 8)
	assertEqual(t, bitsOf(&src), bitsOf(&b))
	assertEqual(t, 8, src.len())
}
