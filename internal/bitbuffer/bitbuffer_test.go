package bitbuffer

import (
	"testing"
)

// bufFromBits builds a Buffer from a sequence of bits.
func bufFromBits(bits ...bool) Buffer {
	var b Buffer
	for _, bit := range bits {
		b.AppendBit(bit)
	}
	return b
}

// bitsOf reads a Buffer back into a []bool for comparison.
func bitsOf(b *Buffer) []bool {
	out := make([]bool, b.Len())
	for i := range out {
		out[i] = b.GetBit(i)
	}
	return out
}

func TestBitBuffer_AppendBitAndGet(t *testing.T) {
	var b Buffer
	if b.GetBit(0) {
		t.Fatal("empty buffer should read 0 out of range")
	}

	pattern := []bool{true, false, true, true, false, false, true, false, true}
	for _, bit := range pattern {
		b.AppendBit(bit)
	}
	if b.Len() != len(pattern) {
		t.Fatalf("len = %d, want %d", b.Len(), len(pattern))
	}
	assertEqual(t, pattern, bitsOf(&b))
	if b.GetBit(len(pattern)) {
		t.Fatal("read past end should be 0")
	}
}

func TestBitBuffer_AppendBits(t *testing.T) {
	var b Buffer
	b.AppendBits(5, 3)
	b.AppendBits(0, 4)
	b.AppendBits(0x7FFFFFFF, 31)
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
			var b Buffer
			b.AppendBits(tc.val, tc.length)
		}()
	}
}

func TestBitBuffer_AppendBuffer(t *testing.T) {
	cases := []struct {
		name     string
		a, b     Buffer
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
			tt.a.AppendBuffer(&tt.b)
			assertEqual(t, tt.wantBits, bitsOf(&tt.a))
			// Bits appended after a fast-path copy must land correctly.
			tt.a.AppendBit(true)
			if !tt.a.GetBit(tt.a.Len() - 1) {
				t.Error("bit appended after appendBuffer is lost")
			}
		})
	}
}

func TestBitBuffer_AppendBufferDoesNotAlias(t *testing.T) {
	src := bufFromBits(true, false, true, true, false, true, true, false)
	var a, b Buffer
	a.AppendBuffer(&src)
	b.AppendBuffer(&src)
	a.AppendBits(0xFF, 8)
	assertEqual(t, bitsOf(&src), bitsOf(&b))
	assertEqual(t, 8, src.Len())
}
