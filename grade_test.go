package qr

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
)

func mustGrade(t *testing.T, img image.Image, opts ...DecodeOption) *Quality {
	t.Helper()
	q, err := Grade(img, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestGradeRendered(t *testing.T) {
	// A rendering is as good as a print can be.
	for _, ver := range []int{1, 7, 25} {
		code := mustEncode(t, "grade me", WithVersionRange(ver, ver))
		q := mustGrade(t, mustImage(t, code, WithScale(8)))
		if q.Grade != GradeA {
			t.Errorf("version %d: grade %v: %+v", ver, q.Grade, q)
		}
		if math.Abs(q.SymbolContrast.Value-1) > 1e-9 || math.Abs(q.Modulation.Value-1) > 1e-9 || math.Abs(q.ReflectanceMargin.Value-1) > 1e-9 {
			t.Errorf("version %d: contrast %v, modulation %v, margin %v; want 1", ver,
				q.SymbolContrast.Value, q.Modulation.Value, q.ReflectanceMargin.Value)
		}
		if q.FixedPatternDamage.Value != 0 || q.AxialNonuniformity.Value > 1e-6 || q.GridNonuniformity.Value > 0.01 {
			t.Errorf("version %d: damage %v, axial %v, grid %v", ver,
				q.FixedPatternDamage.Value, q.AxialNonuniformity.Value, q.GridNonuniformity.Value)
		}
		if math.Abs(q.PixelsPerModule-8) > 0.01 {
			t.Errorf("version %d: %v pixels per module, want 8", ver, q.PixelsPerModule)
		}
		assertEqual(t, "grade me", q.Result.Text, "text")
	}
}

func TestGradeContrast(t *testing.T) {
	code := mustEncode(t, "low contrast")
	// sRGB 0xb0 is 43% of white's linear light: a contrast of 0.57.
	gray := color.Gray{0xb0}
	q := mustGrade(t, mustImage(t, code, WithScale(6), WithForeground(gray)))
	if math.Abs(q.SymbolContrast.Value-(1-srgbLinear[0xb0])) > 1e-9 {
		t.Errorf("contrast %v, want %v", q.SymbolContrast.Value, 1-srgbLinear[0xb0])
	}
	assertEqual(t, GradeB, q.SymbolContrast.Grade, "contrast grade")
	assertEqual(t, GradeB, q.Grade, "grade")
	// Modulation is relative to the contrast: the modules are as even as
	// ever.
	assertEqual(t, GradeA, q.Modulation.Grade, "modulation grade")
}

func TestGradeFinderDamage(t *testing.T) {
	code := mustEncode(t, "finder damage", WithECC(ECCHigh))
	const s = 6
	img := mustImage(t, code, WithScale(s))
	// Two modules of the top-right finder's ring, and one of the
	// bottom-left's, turned light.
	qz := defaultQuietZone
	for _, m := range [][2]int{{code.size - 7, 0}, {code.size - 7, 1}, {0, code.size - 3}} {
		r := image.Rect((qz+m[0])*s, (qz+m[1])*s, (qz+m[0]+1)*s, (qz+m[1]+1)*s)
		draw.Draw(img, r, image.White, image.Point{}, draw.Src)
	}
	q := mustGrade(t, img)
	assertEqual(t, GradeC, q.FixedPatternDamage.Grade, "finder damage grade")
	assertEqual(t, 3.0/192, q.FixedPatternDamage.Value, "share of finder modules wrong")
	assertEqual(t, GradeC, q.Grade, "grade")
	assertEqual(t, GradeA, q.UnusedErrorCorrection.Grade, "unused error correction grade")
}

func TestGradeDataDamage(t *testing.T) {
	code := mustEncode(t, "https://example.com/damaged/data", WithECC(ECCMedium))
	const s = 6
	img := mustImage(t, code, WithScale(s))
	clean := mustGrade(t, img)
	draw.Draw(img, image.Rect(14*s, 14*s, 20*s, 20*s), image.White, image.Point{}, draw.Src)
	q := mustGrade(t, img)
	if q.Result.ErrorsCorrected == 0 {
		t.Fatal("no codeword damaged")
	}
	if q.UnusedErrorCorrection.Value >= clean.UnusedErrorCorrection.Value || q.UnusedErrorCorrection.Value != q.Result.UnusedErrorCorrection {
		t.Errorf("unused error correction %v, clean %v", q.UnusedErrorCorrection.Value, clean.UnusedErrorCorrection.Value)
	}
	// The light modules that should be dark are on the wrong side of the
	// threshold, but as far from it as any.
	if q.ReflectanceMargin.Value > -0.99 || q.Modulation.Value < 0.99 {
		t.Errorf("margin %v, modulation %v", q.ReflectanceMargin.Value, q.Modulation.Value)
	}
	// Erased, the damaged codewords cost half the error correction they
	// cost as errors.
	if q.ReflectanceMargin.Grade < q.UnusedErrorCorrection.Grade || q.Modulation.Grade != GradeA || q.Grade != q.UnusedErrorCorrection.Grade {
		t.Errorf("grade %v: margin %v, modulation %v, unused error correction %v",
			q.Grade, q.ReflectanceMargin.Grade, q.Modulation.Grade, q.UnusedErrorCorrection.Grade)
	}
}

func TestGradeOrientation(t *testing.T) {
	code := mustEncode(t, "turned over", WithVersionRange(3, 3))
	img := mustImage(t, code, WithScale(7))
	for name, im := range map[string]image.Image{
		"mirrored": mirror(img),
		"inverted": mustImage(t, code, WithScale(7), WithForeground(color.White), WithBackground(color.Black)),
		"rotated":  rotateGray(img, 0.3),
	} {
		q := mustGrade(t, im)
		if q.Grade < GradeB || q.FixedPatternDamage.Value != 0 || q.GridNonuniformity.Value > 0.1 {
			t.Errorf("%s: %+v", name, q)
		}
	}
	if q := mustGrade(t, mirror(img)); !q.Result.Mirrored || q.Grade != GradeA {
		t.Errorf("mirrored: grade %v, mirrored %v", q.Grade, q.Result.Mirrored)
	}
}

func TestGradeInPhoto(t *testing.T) {
	// A small symbol in a large image, which the search locates again at
	// full resolution in a window around it.
	img := benchPhoto(2400, 1800, true)
	q := mustGrade(t, img)
	if q.FixedPatternDamage.Value != 0 || math.Abs(q.PixelsPerModule-3) > 0.1 || q.Modulation.Grade < GradeB {
		t.Errorf("%+v", q)
	}
	// Corners are in the image's coordinates, as Decode reports them.
	sub := img.SubImage(image.Rect(100, 100, 2400, 1800))
	q2 := mustGrade(t, sub)
	res, err := Decode(sub)
	assertNoError(t, err)
	assertEqual(t, res.Corners, q2.Result.Corners, "corners")
}

func TestGradeSoftOnly(t *testing.T) {
	code, err := Encode("https://example.com/glare", WithECC(ECCLow), WithoutECCBoost())
	assertNoError(t, err)
	side := float64((code.Size() + 8) * 6)
	img := glareImage(code, 6, side*0.55, side*0.5, side*0.16, 0.85, 2.5)
	if _, err := Grade(img); !errors.Is(err, ErrDecodeFailed) {
		t.Errorf("err = %v, want ErrDecodeFailed", err)
	}
	if _, err := Grade(image.NewGray(image.Rect(0, 0, 100, 100))); !errors.Is(err, ErrNotFound) {
		t.Errorf("blank image: err = %v, want ErrNotFound", err)
	}
}

func TestCodewordMapGrade(t *testing.T) {
	// Version 5-Q: four blocks of 18 check codewords, interleaved.
	m := newCodewordMap(5, ECCQuartile)
	vals := func(grades map[int]float64) []float64 {
		dim := len(m.bitAt)
		v := make([]float64, dim*dim)
		for y, row := range m.bitAt {
			for x, bit := range row {
				v[y*dim+x] = 1
				if g, ok := grades[int(bit)/8]; ok && bit >= 0 {
					v[y*dim+x] = g
				}
			}
		}
		return v
	}
	tests := []struct {
		codewords map[int]float64 // raw codeword: the value of its modules
		want      QualityGrade
	}{
		{nil, GradeA},
		// Codewords 0, 4 and 8 are in block 0: erased at level A, they
		// leave 1 - 3/18 = 0.83 unused, grade A.
		{map[int]float64{0: 0.45, 4: 0.45, 8: 0.45}, GradeA},
		// Seven of grade B leave 0.61 at level A, grade B, and none erased
		// at level B.
		{map[int]float64{0: 0.45, 4: 0.45, 8: 0.45, 12: 0.45, 16: 0.45, 20: 0.45, 24: 0.45}, GradeB},
		// Seven erasures in block 1 leave 0.61: grade B at every level.
		{map[int]float64{1: 0.1, 5: 0.1, 9: 0.1, 13: 0.1, 17: 0.1, 21: 0.1, 25: 0.1}, GradeB},
		// Twelve leave 0.33, grade D, while the level is above D.
		{map[int]float64{1: 0.1, 5: 0.1, 9: 0.1, 13: 0.1, 17: 0.1, 21: 0.1, 25: 0.1, 29: 0.1, 33: 0.1, 37: 0.1, 41: 0.1, 45: 0.1}, GradeD},
		// Below C but above D, they leave the D level at grade A.
		{map[int]float64{1: 0.25, 5: 0.25, 9: 0.25, 13: 0.25, 17: 0.25, 21: 0.25, 25: 0.25, 29: 0.25, 33: 0.25, 37: 0.25, 41: 0.25, 45: 0.25}, GradeD},
	}
	for i, tt := range tests {
		if got := m.grade(vals(tt.codewords)); got != tt.want {
			t.Errorf("case %d: grade %v, want %v", i, got, tt.want)
		}
	}
	for _, c := range []int{0, 4, 8} {
		assertEqual(t, 0, m.block[c], "block")
	}
}

func TestQualityGradeString(t *testing.T) {
	for g, want := range map[QualityGrade]string{GradeA: "A", GradeB: "B", GradeC: "C", GradeD: "D", GradeF: "F", 7: "QualityGrade(7)"} {
		assertEqual(t, want, g.String(), "String")
	}
}

// TestGradeOptions expects Grade to search as Decode does with the same
// options: light symbols on dark are graded unless WithoutInverted skips
// them.
func TestGradeOptions(t *testing.T) {
	code := mustEncode(t, "options")
	light := mustImage(t, code, WithScale(6), WithForeground(color.White), WithBackground(color.Black))
	q := mustGrade(t, light, WithFastPathOnly())
	if !q.Result.Inverted || q.Grade != GradeA {
		t.Errorf("fast path: grade %v, inverted %v", q.Grade, q.Result.Inverted)
	}
	if _, err := Grade(light, WithoutInverted()); !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrDecodeFailed) {
		t.Errorf("WithoutInverted: error %v, want no symbol", err)
	}
}
