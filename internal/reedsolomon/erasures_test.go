package reedsolomon

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"
)

func randomCodeword(rng *rand.Rand, dataLen, eccLen int) []byte {
	data := make([]byte, dataLen)
	rng.Read(data)
	div, _ := Divisor(eccLen)
	return append(data, Remainder(data, div)...)
}

func corrupt(rng *rand.Rand, cw []byte, positions []int) {
	for _, p := range positions {
		cw[p] ^= byte(1 + rng.Intn(255))
	}
}

func TestCorrectErasuresWithinBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 20000; trial++ {
		eccLen := []int{7, 10, 13, 16, 22, 26, 30}[rng.Intn(7)]
		dataLen := 1 + rng.Intn(120)
		orig := randomCodeword(rng, dataLen, eccLen)
		n := len(orig)
		s := rng.Intn(eccLen + 1)
		e := rng.Intn((eccLen-s)/2 + 1)
		perm := rng.Perm(n)
		erasures, errs := perm[:s], perm[s:s+e]
		cw := append([]byte(nil), orig...)
		// Erased bytes may or may not be wrong.
		for _, p := range erasures {
			if rng.Intn(4) > 0 {
				cw[p] ^= byte(1 + rng.Intn(255))
			}
		}
		corrupt(rng, cw, errs)
		if err := CorrectErasures(cw, eccLen, erasures, eccLen); err != nil {
			t.Fatalf("trial %d: n=%d ecc=%d s=%d e=%d: %v", trial, n, eccLen, s, e, err)
		}
		if !bytes.Equal(cw, orig) {
			t.Fatalf("trial %d: n=%d ecc=%d s=%d e=%d: wrong codeword", trial, n, eccLen, s, e)
		}
	}
}

func TestCorrectErasuresMatchesCorrect(t *testing.T) {
	// Without erasures it corrects what Correct corrects.
	rng := rand.New(rand.NewSource(2))
	for trial := 0; trial < 5000; trial++ {
		eccLen := 2 + rng.Intn(29)
		orig := randomCodeword(rng, 1+rng.Intn(100), eccLen)
		e := rng.Intn(eccLen/2 + 1)
		cw := append([]byte(nil), orig...)
		corrupt(rng, cw, rng.Perm(len(cw))[:e])
		if err := CorrectErasures(cw, eccLen, nil, eccLen); err != nil || !bytes.Equal(cw, orig) {
			t.Fatalf("trial %d: e=%d of %d: %v", trial, e, eccLen, err)
		}
	}
}

func TestCorrectErasuresBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	orig := randomCodeword(rng, 20, 10)
	cw := append([]byte(nil), orig...)
	corrupt(rng, cw, []int{1, 5, 9})
	// 3 errors and 2 erasures need 8; a budget of 7 refuses.
	if err := CorrectErasures(append([]byte(nil), cw...), 10, []int{12, 14}, 7); !errors.Is(err, ErrUncorrectable) {
		t.Errorf("budget 7: error %v", err)
	}
	if err := CorrectErasures(cw, 10, []int{12, 14}, 8); err != nil || !bytes.Equal(cw, orig) {
		t.Errorf("budget 8: %v", err)
	}
}

func TestCorrectErasuresRefusesBadPositions(t *testing.T) {
	cw := randomCodeword(rand.New(rand.NewSource(4)), 10, 8)
	damaged := append([]byte(nil), cw...)
	damaged[0] ^= 1
	// Refused whether or not the block needs correcting.
	for name, block := range map[string][]byte{"intact": cw, "damaged": damaged} {
		for _, erasures := range [][]int{{3, 3}, {-1}, {len(cw)}} {
			if err := CorrectErasures(append([]byte(nil), block...), 8, erasures, 8); !errors.Is(err, ErrUncorrectable) {
				t.Errorf("%s block, erasures %v: error %v", name, erasures, err)
			}
		}
	}
}

// FuzzCorrectErasures checks the guarantee callers rely on against
// miscorrection: a block CorrectErasures accepts is a codeword, changed only
// at the erasures and at no more than (budget - erasures) / 2 other
// positions; and a block within the budget is restored.
func FuzzCorrectErasures(f *testing.F) {
	f.Add(int64(1), uint8(10), uint8(20), uint8(3), uint8(2), uint8(8))
	f.Add(int64(2), uint8(30), uint8(100), uint8(0), uint8(20), uint8(30))
	f.Fuzz(func(t *testing.T, seed int64, ecc, data, nerase, nerr, budget uint8) {
		eccLen := 2 + int(ecc)%29
		dataLen := 1 + int(data)%150
		rng := rand.New(rand.NewSource(seed))
		orig := randomCodeword(rng, dataLen, eccLen)
		n := len(orig)
		perm := rng.Perm(n)
		s := min(int(nerase)%(eccLen+2), n)
		e := min(int(nerr)%(eccLen+2), n-s)
		erasures, errs := perm[:s], perm[s:s+e]
		got := append([]byte(nil), orig...)
		for _, p := range erasures {
			got[p] ^= byte(rng.Intn(256))
		}
		corrupt(rng, got, errs)
		received := append([]byte(nil), got...)
		b := int(budget) % (eccLen + 1)
		err := CorrectErasures(got, eccLen, erasures, b)
		if err != nil {
			if 2*e+s <= b {
				t.Fatalf("s=%d e=%d budget %d of %d: %v", s, e, b, eccLen, err)
			}
			return
		}
		div, _ := Divisor(eccLen)
		if !bytes.Equal(Remainder(got[:dataLen], div), got[dataLen:]) {
			t.Fatal("accepted a block that is not a codeword")
		}
		erased := map[int]bool{}
		for _, p := range erasures {
			erased[p] = true
		}
		other := 0
		for i := range got {
			if got[i] != received[i] && !erased[i] {
				other++
			}
		}
		if 2*other+s > b {
			t.Fatalf("s=%d budget %d: changed %d positions besides the erasures", s, b, other)
		}
		if 2*e+s <= b && !bytes.Equal(got, orig) {
			t.Fatal("within the budget but not restored")
		}
	})
}

func TestCorrectErasuresBeyondCapacity(t *testing.T) {
	// Blocks corrupted beyond repair must be refused, or accepted only as a
	// codeword within the budget of the received word: what keeps erasure
	// decoding from inventing data.
	rng := rand.New(rand.NewSource(5))
	refused := 0
	for trial := 0; trial < 20000; trial++ {
		eccLen := 2 + rng.Intn(29)
		dataLen := 1 + rng.Intn(60)
		orig := randomCodeword(rng, dataLen, eccLen)
		n := len(orig)
		perm := rng.Perm(n)
		s := rng.Intn(min(eccLen, n) + 1)
		e := min(n-s, (eccLen-s)/2+1+rng.Intn(eccLen+1))
		erasures := perm[:s]
		got := append([]byte(nil), orig...)
		for _, p := range erasures {
			got[p] ^= byte(rng.Intn(256))
		}
		corrupt(rng, got, perm[s:s+e])
		received := append([]byte(nil), got...)
		budget := eccLen
		if rng.Intn(2) == 0 {
			budget = s + rng.Intn(eccLen-s+1)
		}
		if err := CorrectErasures(got, eccLen, erasures, budget); err != nil {
			refused++
			continue
		}
		div, _ := Divisor(eccLen)
		if !bytes.Equal(Remainder(got[:dataLen], div), got[dataLen:]) {
			t.Fatalf("trial %d: accepted a block that is not a codeword", trial)
		}
		erased := map[int]bool{}
		for _, p := range erasures {
			erased[p] = true
		}
		other := 0
		for i := range got {
			if got[i] != received[i] && !erased[i] {
				other++
			}
		}
		if 2*other+s > budget {
			t.Fatalf("trial %d: %d changes besides %d erasures exceed budget %d", trial, other, s, budget)
		}
	}
	if refused < 10000 {
		t.Errorf("only %d of 20000 blocks beyond repair refused", refused)
	}
	cw := randomCodeword(rng, 10, 6)
	if err := CorrectErasures(cw, 6, []int{0, 1, 2, 3}, 3); !errors.Is(err, ErrUncorrectable) {
		t.Errorf("more erasures than the budget: %v", err)
	}
}
