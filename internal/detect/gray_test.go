package detect

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestThresholdMapRows(t *testing.T) {
	// A width that is not a multiple of the block size exercises the last
	// block, which is aligned to the right edge and overlaps its neighbor.
	r := rand.New(rand.NewSource(1))
	w, h := 83, 61
	l := make([]uint8, w*h)
	for i := range l {
		l[i] = uint8(r.Intn(256))
	}
	tm := HybridThresholds(l, w, h)
	for y := 0; y < h; y++ {
		// runs must agree with dark pixel by pixel.
		runs := tm.runs(y, false, nil)
		x, dark := 0, false
		for i, n := range runs {
			if i > 0 && n == 0 {
				t.Fatalf("row %d: empty run %d in %v", y, i, runs)
			}
			for k := 0; k < n; k++ {
				if got := tm.Dark(x, y); got != dark {
					t.Fatalf("row %d, x %d: runs say dark=%v, dark() says %v", y, x, dark, got)
				}
				x++
			}
			dark = !dark
		}
		if x != w {
			t.Fatalf("row %d: runs cover %d pixels, want %d", y, x, w)
		}
	}
	if tm.Dark(-1, 0) || tm.Dark(0, -1) || tm.Dark(w, 0) || tm.Dark(0, h) {
		t.Error("pixels outside the image are dark")
	}
	inv := tm.runs(0, true, nil)
	if inv[0] != 0 && tm.Dark(0, 0) == false {
		t.Errorf("inverted row starts with a light run %d at a light pixel", inv[0])
	}
}

func TestRangePyramid(t *testing.T) {
	// A 10×10-block image, flat except for one block of contrast at (8, 8).
	lo := make([]uint8, 100)
	hi := make([]uint8, 100)
	for i := range lo {
		lo[i], hi[i] = 200, 200
	}
	lo[88], hi[88] = 10, 240
	p := newRangePyramid(10, 10, func(i int) (uint8, uint8) { return lo[i], hi[i] })
	if _, _, ok := p.contrast(1, 1, 24); !ok {
		t.Fatal("no contrast found from the far corner")
	}
	l, h, _ := p.contrast(8, 8, 24)
	if l != 10 || h != 240 {
		t.Errorf("contrast next to the block = %d..%d, want 10..240", l, h)
	}
	flat := newRangePyramid(4, 4, func(int) (uint8, uint8) { return 100, 110 })
	if _, _, ok := flat.contrast(0, 0, 24); ok {
		t.Error("contrast found in a flat image")
	}
}

func TestHalve(t *testing.T) {
	l := []uint8{0, 4, 8, 12, 100, 104, 108, 112, 255, 255}
	got, w, h := Halve(l, 5, 2)
	if w != 2 || h != 1 || len(got) != 2 {
		t.Fatalf("Halve: %dx%d %v", w, h, got)
	}
	if got[0] != (0+4+104+108+2)/4 || got[1] != (8+12+112+255+2)/4 {
		t.Errorf("Halve = %v", got)
	}
}

// referenceHalve is Halve up to v2.7, which added one pixel at a time.
func referenceHalve(l []uint8, w, h int) ([]uint8, int, int) {
	nw, nh := w/2, h/2
	out := make([]uint8, nw*nh)
	for y := 0; y < nh; y++ {
		r0, r1 := l[2*y*w:], l[(2*y+1)*w:]
		for x := 0; x < nw; x++ {
			out[y*nw+x] = uint8((int(r0[2*x]) + int(r0[2*x+1]) + int(r1[2*x]) + int(r1[2*x+1]) + 2) / 4)
		}
	}
	return out, nw, nh
}

func TestHalveMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	fill := map[string]func(i int) uint8{
		"random": func(int) uint8 { return uint8(r.Intn(256)) },
		"white":  func(int) uint8 { return 255 },
		"black":  func(int) uint8 { return 0 },
		// Sums of 1, 2 and 3 modulo 4 round both ways.
		"steps": func(i int) uint8 { return uint8(i % 4 * 85) },
	}
	for name, f := range fill {
		for _, sz := range [][2]int{{1, 1}, {2, 2}, {7, 3}, {8, 2}, {9, 5}, {15, 4}, {16, 2}, {16, 16}, {17, 9}, {33, 4}, {83, 61}, {4001, 3}} {
			w, h := sz[0], sz[1]
			l := make([]uint8, w*h)
			for i := range l {
				l[i] = f(i)
			}
			got, gw, gh := Halve(l, w, h)
			want, ww, wh := referenceHalve(l, w, h)
			if gw != ww || gh != wh || !bytes.Equal(got, want) {
				t.Errorf("%s %dx%d: Halve differs from the reference", name, w, h)
			}
		}
	}
}

func BenchmarkHalve(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	l := make([]uint8, 4000*3000)
	for i := range l {
		l[i] = uint8(r.Intn(256))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Halve(l, 4000, 3000)
	}
}

func TestFinderRowStep(t *testing.T) {
	for _, c := range []struct{ px, want int }{{640 * 480, 1}, {1920 * 1080, 2}, {4032 * 3024, 3}} {
		if got := FinderRowStep(c.px); got != c.want {
			t.Errorf("FinderRowStep(%d) = %d, want %d", c.px, got, c.want)
		}
	}
}
