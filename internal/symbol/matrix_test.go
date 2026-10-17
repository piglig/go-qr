package symbol

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/piglig/go-qr/v2/internal/errs"
)

// rawCodewords returns the interleaved codewords of a symbol of a version
// and level holding random data, as ReadMatrix reads them.
func rawCodewords(t *testing.T, ver int, ecc Level) []byte {
	t.Helper()
	data := make([]byte, NumDataCodewords(ver, ecc))
	rand.New(rand.NewSource(int64(ver))).Read(data)
	raw, err := NewBuilder(ver, ecc).AddECCAndInterleave(data)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDeinterleaveCountsErrorCorrection(t *testing.T) {
	tests := []struct {
		name    string
		ver     int
		ecc     Level
		corrupt []int // raw codeword positions
		soft    bool
		want    float64
	}{
		// Version 1-M: one block of 10 check codewords, 2 of them misdecode
		// protection, so a capacity of 8.
		{name: "clean", ver: 1, ecc: Medium, want: 1},
		{name: "three errors", ver: 1, ecc: Medium, corrupt: []int{0, 5, 20}, want: 1 - 6.0/8},
		// 1-L: capacity 4; three errors are corrected into the misdecode
		// protection codewords, which leaves none unused.
		{name: "beyond capacity", ver: 1, ecc: Low, corrupt: []int{1, 2, 3}, want: 0},
		// 5-Q: four blocks of 18 check codewords, interleaved; the block
		// with two errors counts.
		{name: "worst block", ver: 5, ecc: Quartile, corrupt: []int{0, 4, 1}, want: 1 - 4.0/18},
		// Four errors exceed the soft attempt's budget of 6 by hard
		// decisions; two are erased and two corrected.
		{name: "erasures", ver: 1, ecc: Medium, corrupt: []int{0, 5, 20, 25}, soft: true, want: 1 - 6.0/8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := rawCodewords(t, tt.ver, tt.ecc)
			want, err := deinterleaveAndCorrect(raw, tt.ver, tt.ecc, nil)
			if err != nil {
				t.Fatal(err)
			}
			var rel []float32
			if tt.soft {
				rel = make([]float32, len(raw))
				for i := range rel {
					rel[i] = 1
				}
			}
			for _, i := range tt.corrupt {
				raw[i] ^= 0xa5
				if rel != nil {
					rel[i] = 0.01
				}
			}
			got, err := deinterleaveAndCorrect(raw, tt.ver, tt.ecc, rel)
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Codewords) != string(want.Codewords) {
				t.Fatal("corrected data differs")
			}
			if got.Corrected != len(tt.corrupt) {
				t.Errorf("corrected = %d, want %d", got.Corrected, len(tt.corrupt))
			}
			if math.Abs(got.UnusedEC-tt.want) > 1e-9 {
				t.Errorf("unused error correction = %v, want %v", got.UnusedEC, tt.want)
			}
		})
	}
}

// TestReadMatrixRejectsSizes expects grids of no symbol's size to fail
// before their format information is read.
func TestReadMatrixRejectsSizes(t *testing.T) {
	for _, n := range []int{0, 20, 22, 4*(MaxVersion+1) + 17} {
		grid := make([][]bool, n)
		for y := range grid {
			grid[y] = make([]bool, n)
		}
		if _, err := ReadMatrix(grid, nil); !errors.Is(err, errs.ErrDecodeFailed) {
			t.Errorf("%d×%d grid: error %v, want ErrDecodeFailed", n, n, err)
		}
	}
}
