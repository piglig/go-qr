package symbol

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/piglig/go-qr/v2/internal/errs"
)

// zigzagCodewords is drawCodewords up to v2.6, which walked the zig-zag
// itself. It fills the non-function modules with the given codeword bytes
// following the QR Code zig-zag traversal.
func (q *Builder) zigzagCodewords(data []byte) error {
	n := NumRawDataModules(q.Version) / 8
	if len(data) != n {
		return fmt.Errorf("%w: codeword length mismatch", errs.ErrInvalidArgument)
	}

	i := 0
	for right := q.Size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.Size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := ((right + 1) & 2) == 0
				y := vert
				if upward {
					y = q.Size - 1 - vert
				}
				if !q.IsFunction[y][x] && i < len(data)*8 {
					q.Modules[y][x] = getBit(int(data[i>>3]), 7-(i&7))
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
		data := make([]byte, NumRawDataModules(v)/8)
		rng.Read(data)
		for _, ecc := range []Level{Low, High} {
			want := NewBuilder(v, ecc)
			want.DrawFunctionPatterns()
			if err := want.zigzagCodewords(data); err != nil {
				t.Fatal(err)
			}
			got := NewBuilder(v, ecc)
			got.DrawFunctionPatterns()
			if err := got.DrawCodewords(data); err != nil {
				t.Fatal(err)
			}
			for y := range want.Modules {
				for x := range want.Modules[y] {
					if got.Modules[y][x] != want.Modules[y][x] {
						t.Fatalf("version %d: module (%d, %d) differs", v, x, y)
					}
				}
			}
			// The packed rows are the modules, format bits aside: the
			// template's function rows have those of mask 0 at level L,
			// which mask selection redraws.
			ref := newPackedModules(got.Size)
			ref.pack(got.Modules)
			for y := 0; y < got.Size; y++ {
				line := got.packed[y]
				for x := 0; x < got.Size; x++ {
					if x != 8 && y != 8 && line.dark(x) != got.Modules[y][x] {
						t.Fatalf("version %d: packed module (%d, %d) differs", v, x, y)
					}
				}
				if line.dark(got.Size) {
					t.Fatalf("version %d: packed row %d has bits past the side", v, y)
				}
			}
		}
	}
	if err := NewBuilder(1, Low).DrawCodewords(make([]byte, 3)); err == nil {
		t.Error("drawCodewords accepted the wrong number of codewords")
	}
}
