package reedsolomon

import (
	"bytes"
	"errors"
	"testing"
)

// encode builds a full codeword [data | ecc] for the given ECC length.
func encode(t *testing.T, data []byte, eccLen int) []byte {
	t.Helper()
	div, err := Divisor(eccLen)
	if err != nil {
		t.Fatalf("Divisor(%d): %v", eccLen, err)
	}
	ecc := Remainder(data, div)
	if len(ecc) != eccLen {
		t.Fatalf("ecc len = %d, want %d", len(ecc), eccLen)
	}
	return append(append([]byte{}, data...), ecc...)
}

func TestDivisor_RangeAndCache(t *testing.T) {
	if _, err := Divisor(0); !errors.Is(err, ErrInvalidDegree) {
		t.Errorf("Divisor(0) error = %v, want ErrInvalidDegree", err)
	}
	if _, err := Divisor(256); !errors.Is(err, ErrInvalidDegree) {
		t.Errorf("Divisor(256) error = %v, want ErrInvalidDegree", err)
	}
	// Cached and freshly-computed divisors must agree.
	cached, _ := Divisor(10)
	fresh, _ := computeDivisor(10)
	if !bytes.Equal(cached, fresh) {
		t.Errorf("cached divisor != computed: %v vs %v", cached, fresh)
	}
}

func TestCorrect_CleanCodeword(t *testing.T) {
	data := []byte("hello reed-solomon")
	block := encode(t, data, 10)
	if err := Correct(block, 10); err != nil {
		t.Fatalf("Correct on clean codeword: %v", err)
	}
	if !bytes.Equal(block[:len(data)], data) {
		t.Errorf("data changed: %q", block[:len(data)])
	}
}

func TestCorrect_RepairsWithinCapacity(t *testing.T) {
	data := []byte("the quick brown fox")
	const eccLen = 10 // corrects up to eccLen/2 = 5 byte errors
	orig := encode(t, data, eccLen)

	block := append([]byte{}, orig...)
	// Corrupt 5 bytes at assorted positions.
	for _, pos := range []int{0, 3, 7, len(block) - 1, len(block) - 4} {
		block[pos] ^= 0x5A
	}

	if err := Correct(block, eccLen); err != nil {
		t.Fatalf("Correct within capacity: %v", err)
	}
	if !bytes.Equal(block, orig) {
		t.Errorf("repair mismatch:\n got %v\nwant %v", block, orig)
	}
}

func TestCorrect_BeyondCapacity(t *testing.T) {
	data := []byte("data beyond repair")
	const eccLen = 6 // corrects up to 3 errors
	block := encode(t, data, eccLen)
	for i := 0; i < 6; i++ { // 6 errors > capacity
		block[i] ^= 0xFF
	}
	if err := Correct(block, eccLen); !errors.Is(err, ErrUncorrectable) {
		t.Errorf("Correct beyond capacity error = %v, want ErrUncorrectable", err)
	}
}
