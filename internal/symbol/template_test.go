package symbol

import (
	"math/rand"
	"testing"
)

func TestCodewordMapMatchesPlacement(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for _, ver := range []int{1, 2, 7, 14, 27, 40} {
		data := make([]byte, NumRawDataModules(ver)/8)
		rng.Read(data)
		symbol := NewBuilder(ver, Medium)
		symbol.DrawFunctionPatterns()
		if err := symbol.DrawCodewords(data); err != nil {
			t.Fatal(err)
		}
		read := func(modules [][]bool) []byte {
			b := NewBuilder(ver, Medium)
			b.DrawFunctionPatterns()
			for y := range modules {
				copy(b.Modules[y], modules[y])
			}
			return b.ReadCodewords()
		}
		orig := read(symbol.Modules)
		bitAt := BitAt(ver)
		for _, k := range []int{0, len(orig) / 2, len(orig) - 1} {
			flipped := make([][]bool, symbol.Size)
			for y := range flipped {
				flipped[y] = append([]bool(nil), symbol.Modules[y]...)
				for x := range flipped[y] {
					if bit := bitAt[y][x]; bit >= 0 && int(bit)/8 == k {
						flipped[y][x] = !flipped[y][x]
					}
				}
			}
			got := read(flipped)
			for i := range got {
				if (got[i] != orig[i]) != (i == k) {
					t.Fatalf("version %d: flipping codeword %d changed codeword %d", ver, k, i)
				}
			}
			if got[k] != ^orig[k] {
				t.Fatalf("version %d: codeword %d not fully covered by the map", ver, k)
			}
		}
	}
}

// TestCorrectableErrors checks the error correction capacity per block
// against ISO/IEC 18004 Table 9, whose smallest symbols correct fewer
// errors than half their error correction codewords.
func TestCorrectableErrors(t *testing.T) {
	want := map[[2]int]int{
		{1, int(Low)}: 2, {1, int(Medium)}: 4, {1, int(Quartile)}: 6, {1, int(High)}: 8,
		{2, int(Low)}: 4, {2, int(Medium)}: 8, {2, int(Quartile)}: 11, {2, int(High)}: 14,
		{3, int(Low)}: 7, {3, int(Medium)}: 13, {3, int(Quartile)}: 9, {3, int(High)}: 11,
	}
	for ver := MinVersion; ver <= MaxVersion; ver++ {
		for _, ecc := range []Level{Low, Medium, Quartile, High} {
			n, ok := want[[2]int{ver, int(ecc)}]
			if !ok {
				n = int(ECCCodewordsPerBlock[ecc][ver]) / 2
			}
			if got := CorrectableErrors(ver, ecc); got != n {
				t.Errorf("version %d level %d: %d, want %d", ver, ecc, got, n)
			}
		}
	}
}
