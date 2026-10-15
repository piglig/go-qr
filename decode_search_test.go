package qr

import (
	"image"
	"slices"
	"testing"
)

func TestSearchOrder(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want []int
	}{
		{164, 164, []int{0}},               // too small to halve
		{640, 480, []int{0, 1}},            // under searchPixels: from full resolution
		{2400, 1800, []int{2, 3, 1, 0}},    // 4.3 MP: from a quarter
		{4000, 3000, []int{3, 4, 2, 1, 0}}, // 12 MP: from an eighth
	} {
		if got := searchOrder(tc.w, tc.h); !slices.Equal(got, tc.want) {
			t.Errorf("searchOrder(%d, %d) = %v, want %v", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestRegion(t *testing.T) {
	// A version 4 symbol (33 modules) at level 1, its modules 1.5 pixels
	// there: finder centers 26 modules apart.
	s := &symbolSearch{scale: 2, lw: 4000, lh: 3000}
	f := func(x, y, m float64) finderPattern { return finderPattern{x: x, y: y, moduleSize: m} }
	tl, tr, bl := f(500, 500, 1.5), f(539, 500, 1.5), f(500, 539, 1.5)
	r, ok := s.region([3]finderPattern{tl, tr, bl})
	if !ok {
		t.Fatal("no region for a barely resolved triple")
	}
	// Whichever finder is the corner one, the fourth corner is one of
	// (539, 539), (461, 539) and (539, 461): x and y span 461 to 539, padded
	// by 8 modules of 1.5 pixels, then doubled to full resolution.
	if want := image.Rect(898, 898, 1102, 1102); r != want {
		t.Errorf("region %v, want %v", r, want)
	}
	for name, tri := range map[string][3]finderPattern{
		"modules resolved":  {f(500, 500, 3), f(578, 500, 3), f(500, 578, 3)},
		"beyond version 40": {f(100, 100, 1.2), f(900, 100, 1.2), f(100, 900, 1.2)},
		"window too large":  {f(100, 100, 2.9), f(600, 100, 2.9), f(100, 600, 2.9)},
	} {
		if r, ok := s.region(tri); ok {
			t.Errorf("%s: region %v", name, r)
		}
	}
	// Finders 300 pixels apart with modules of 0.8 imply about 380 modules
	// per side; the window, about 0.5 megapixels, would pass the size cap.
	full := &symbolSearch{scale: 1, lw: 4000, lh: 3000}
	if r, ok := full.region([3]finderPattern{f(1000, 1000, 0.8), f(1300, 1000, 0.8), f(1000, 1300, 0.8)}); ok {
		t.Errorf("finders too far apart for one symbol: region %v", r)
	}
}

func TestDecodeSmallCodeInLargeImage(t *testing.T) {
	// A version 4 code at 3 pixels per module in 4.3 megapixels of texture:
	// located at half resolution, where its modules span 1.5 pixels, and
	// read through a region at full resolution.
	img := benchPhoto(2400, 1800, true)
	res, err := Decode(img)
	assertNoError(t, err)
	assertEqual(t, "https://github.com/piglig/go-qr?ref=bench&v=1", res.Text)
	all, err := DecodeAll(img)
	assertNoError(t, err)
	assertLen(t, all, 1)
	assertEqual(t, res.Corners, all[0].Corners)
	// The symbol lies at (800, 900), 135 pixels wide and slightly rotated.
	for _, c := range res.Corners {
		if !c.In(image.Rect(780, 880, 960, 1060)) {
			t.Errorf("corner %v outside the symbol", c)
		}
	}
}
