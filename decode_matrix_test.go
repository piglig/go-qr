package qr

import (
	"math"
	"testing"
)

// rawCodewords returns the interleaved codewords of code as decodeMatrix
// reads them from its modules.
func rawCodewords(t *testing.T, code *Code) []byte {
	t.Helper()
	b := newBuilder(code.version, code.ecc)
	b.drawFunctionPatterns()
	for y := range b.modules {
		copy(b.modules[y], code.modules[y])
	}
	if err := b.applyMask(code.mask); err != nil {
		t.Fatal(err)
	}
	return b.readCodewords()
}

func TestDeinterleaveCountsErrorCorrection(t *testing.T) {
	tests := []struct {
		name    string
		ver     int
		ecc     ECC
		corrupt []int // raw codeword positions
		soft    bool
		want    float64
	}{
		// Version 1-M: one block of 10 check codewords, 2 of them misdecode
		// protection, so a capacity of 8.
		{name: "clean", ver: 1, ecc: ECCMedium, want: 1},
		{name: "three errors", ver: 1, ecc: ECCMedium, corrupt: []int{0, 5, 20}, want: 1 - 6.0/8},
		// 1-L: capacity 4; three errors are corrected into the misdecode
		// protection codewords, which leaves none unused.
		{name: "beyond capacity", ver: 1, ecc: ECCLow, corrupt: []int{1, 2, 3}, want: 0},
		// 5-Q: four blocks of 18 check codewords, interleaved; the block
		// with two errors counts.
		{name: "worst block", ver: 5, ecc: ECCQuartile, corrupt: []int{0, 4, 1}, want: 1 - 4.0/18},
		// Four errors exceed the soft attempt's budget of 6 by hard
		// decisions; two are erased and two corrected.
		{name: "erasures", ver: 1, ecc: ECCMedium, corrupt: []int{0, 5, 20, 25}, soft: true, want: 1 - 6.0/8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := mustEncode(t, "ERROR CORRECTION", WithECC(tt.ecc), WithVersionRange(tt.ver, tt.ver), WithoutECCBoost())
			raw := rawCodewords(t, code)
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
			if string(got.data) != string(want.data) {
				t.Fatal("corrected data differs")
			}
			if got.corrected != len(tt.corrupt) {
				t.Errorf("corrected = %d, want %d", got.corrected, len(tt.corrupt))
			}
			if math.Abs(got.unusedEC-tt.want) > 1e-9 {
				t.Errorf("unused error correction = %v, want %v", got.unusedEC, tt.want)
			}
		})
	}
}

func TestDecodeReportsErrorCorrection(t *testing.T) {
	code := mustEncode(t, "https://example.com/error-correction", WithECC(ECCHigh))
	img, err := code.Image(WithScale(4))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Decode(img)
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorsCorrected != 0 || res.UnusedErrorCorrection != 1 || res.SoftDecision {
		t.Errorf("clean symbol: corrected %d, unused %v, soft %v", res.ErrorsCorrected, res.UnusedErrorCorrection, res.SoftDecision)
	}

	// A light square over the data region damages a few codewords.
	for y := 60; y < 72; y++ {
		for x := 60; x < 72; x++ {
			img.Set(x, y, img.At(0, 0))
		}
	}
	res, err = Decode(img)
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorsCorrected == 0 || res.UnusedErrorCorrection >= 1 || res.UnusedErrorCorrection <= 0 {
		t.Errorf("damaged symbol: corrected %d, unused %v", res.ErrorsCorrected, res.UnusedErrorCorrection)
	}

	// The soft attempts are reported.
	grid := make([][]bool, code.size)
	for y := range grid {
		grid[y] = append([]bool(nil), code.modules[y]...)
	}
	rel := make([]float32, code.size*code.size)
	if res, err := decodeGrid(grid, rel); err != nil || !res.SoftDecision {
		t.Errorf("soft attempt: %v, %v", res, err)
	}
}
