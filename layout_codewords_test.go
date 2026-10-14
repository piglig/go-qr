package qr

import (
	"fmt"
	"math/rand"
	"testing"
)

// zigzagCodewords is drawCodewords up to v2.6, which walked the zig-zag
// itself. It fills the non-function modules with the given codeword bytes
// following the QR Code zig-zag traversal.
func (q *builder) zigzagCodewords(data []byte) error {
	numRawDataModules := numRawDataModules(q.version) / 8
	if len(data) != numRawDataModules {
		return fmt.Errorf("%w: codeword length mismatch", ErrInvalidArgument)
	}

	i := 0
	for right := q.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := ((right + 1) & 2) == 0
				y := vert
				if upward {
					y = q.size - 1 - vert
				}
				if !q.isFunction[y][x] && i < len(data)*8 {
					q.modules[y][x] = getBit(int(data[i>>3]), 7-(i&7))
					i++
				}
			}
		}
	}
	return nil
}

func TestDrawCodewordsMatchesZigzag(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for v := 1; v <= 40; v++ {
		data := make([]byte, numRawDataModules(v)/8)
		rng.Read(data)
		for _, ecc := range []ECC{ECCLow, ECCHigh} {
			want := newBuilder(v, ecc)
			want.drawFunctionPatterns()
			if err := want.zigzagCodewords(data); err != nil {
				t.Fatal(err)
			}
			got := newBuilder(v, ecc)
			got.drawFunctionPatterns()
			if err := got.drawCodewords(data); err != nil {
				t.Fatal(err)
			}
			for y := range want.modules {
				for x := range want.modules[y] {
					if got.modules[y][x] != want.modules[y][x] {
						t.Fatalf("version %d: module (%d, %d) differs", v, x, y)
					}
				}
			}
			// The packed rows are the modules, format bits aside: the
			// template's function rows have those of mask 0 at level L,
			// which mask selection redraws.
			ref := newPackedModules(got.size)
			ref.pack(got.modules)
			for y := 0; y < got.size; y++ {
				line := got.packed[y]
				for x := 0; x < got.size; x++ {
					if x != 8 && y != 8 && line.dark(x) != got.modules[y][x] {
						t.Fatalf("version %d: packed module (%d, %d) differs", v, x, y)
					}
				}
				if line.dark(got.size) {
					t.Fatalf("version %d: packed row %d has bits past the side", v, y)
				}
			}
		}
	}
	if err := newBuilder(1, ECCLow).drawCodewords(make([]byte, 3)); err == nil {
		t.Error("drawCodewords accepted the wrong number of codewords")
	}
}
