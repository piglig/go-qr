package symbol

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/piglig/go-qr/v2/internal/reedsolomon"
)

func TestSoftCorrectKeepsProtection(t *testing.T) {
	// A version 1-L block has 7 check bytes, of which Table 9 reserves 3:
	// hard decisions correct 3 errors, soft attempts no more than 2 even
	// with every reliability pointing at them.
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		data := make([]byte, 19)
		rng.Read(data)
		div, _ := reedsolomon.Divisor(7)
		orig := append(data, reedsolomon.Remainder(data, div)...)
		cw := append([]byte(nil), orig...)
		pos := rng.Perm(len(cw))[:3]
		rel := make([]float32, len(cw))
		for j := range rel {
			rel[j] = 1
		}
		for _, p := range pos {
			cw[p] ^= byte(1 + rng.Intn(255))
			rel[p] = 0
		}
		budget := 7 - MisdecodeProtection(1, Low) - gmdMargin
		if _, err := softCorrect(append([]byte(nil), cw...), 7, rel, budget); err == nil {
			t.Fatalf("block %d: three errors corrected within budget %d", i, budget)
		}
		if err := reedsolomon.Correct(cw, 7); err != nil {
			t.Fatalf("block %d: hard decisions: %v", i, err)
		}
	}
	// Two errors at the least reliable bytes are corrected by erasing them.
	data := []byte("soft decision deco")
	div, _ := reedsolomon.Divisor(10)
	orig := append(append([]byte(nil), data...), reedsolomon.Remainder(data, div)...)
	cw := append([]byte(nil), orig...)
	rel := make([]float32, len(cw))
	for j := range rel {
		rel[j] = 1
	}
	for _, p := range []int{2, 5, 11, 20} {
		cw[p] ^= 0x5a
		rel[p] = 0.01
	}
	if erased, err := softCorrect(cw, 10, rel, 8); err != nil || string(cw) != string(orig) || erased != nil {
		t.Errorf("four errors within the budget: erased %v, %v", erased, err)
	}
	// Five are not, by hard decisions: two of them are erased.
	cw = append([]byte(nil), orig...)
	for _, p := range []int{2, 5, 11, 20, 23} {
		cw[p] ^= 0x5a
		rel[p] = 0.01
	}
	erased, err := softCorrect(cw, 10, rel, 8)
	if err != nil || string(cw) != string(orig) {
		t.Errorf("five errors at the least reliable bytes: %v", err)
	}
	if len(erased) != 2 || rel[erased[0]] != 0.01 || rel[erased[1]] != 0.01 {
		t.Errorf("erased %v, want two of the least reliable bytes", erased)
	}
}

func TestPilotsAreTheKnownModules(t *testing.T) {
	for ver := MinVersion; ver <= MaxVersion; ver++ {
		b := NewBuilder(ver, Low)
		b.DrawFunctionPatterns()
		known := map[[2]int]bool{}
		for _, m := range Pilots(ver) {
			at := [2]int{int(m.X), int(m.Y)}
			if known[at] || m.Dark != b.Modules[m.Y][m.X] {
				t.Fatalf("version %d, module %v: repeated or wrong value", ver, at)
			}
			known[at] = true
		}
		for y := 0; y < b.Size; y++ {
			for x := 0; x < b.Size; x++ {
				if want := b.IsFunction[y][x] && !IsFormatModule(x, y, b.Size); known[[2]int{x, y}] != want {
					t.Fatalf("version %d, module (%d, %d): known %v, want %v", ver, x, y, !want, want)
				}
			}
		}
	}
}

// TestCodewordReliabilityFollowsPlacement checks the decoder's walk of the
// codeword placement against the encoder's map of the bit each module
// carries.
func TestCodewordReliabilityFollowsPlacement(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for ver := MinVersion; ver <= MaxVersion; ver++ {
		b := NewBuilder(ver, Low)
		b.DrawFunctionPatterns()
		n := b.Size
		rel := make([]float32, n*n)
		for i := range rel {
			rel[i] = rng.Float32()
		}
		want := make([]float32, NumRawDataModules(ver)/8)
		for i := range want {
			want[i] = float32(math.Inf(1))
		}
		for y, row := range bitMap(b) { // not cached, unlike getTemplate's
			for x, bit := range row {
				if bit >= 0 && int(bit)/8 < len(want) {
					want[bit/8] = min(want[bit/8], rel[y*n+x])
				}
			}
		}
		if got := b.CodewordReliability(rel); !slices.Equal(got, want) {
			t.Fatalf("version %d: reliabilities differ", ver)
		}
	}
}
