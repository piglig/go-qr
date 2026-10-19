package detect

import (
	"bytes"
	"math/rand"
	"slices"
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

// referenceBlockStats is how HybridThresholds measured a block up to v2.7,
// a pixel at a time.
func referenceBlockStats(l []uint8, w int) (lo, hi uint8, sum int) {
	mn, mx := 255, 0
	for y := 0; y < ThresholdBlock; y++ {
		for _, v := range l[y*w : y*w+ThresholdBlock] {
			sum += int(v)
			mn, mx = min(mn, int(v)), max(mx, int(v))
		}
	}
	return uint8(mn), uint8(mx), sum
}

func TestBlockStatsMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	const w = 11 // rows wider than a block, so a block reads part of each
	fills := []func() uint8{
		func() uint8 { return uint8(r.Intn(256)) },
		func() uint8 { return 255 },
		func() uint8 { return 0 },
		func() uint8 { return uint8(r.Intn(2) * 255) },
		func() uint8 { return uint8(127 + r.Intn(3)) },
	}
	for i := 0; i < 20000; i++ {
		f := fills[i%len(fills)]
		l := make([]uint8, w*ThresholdBlock)
		for j := range l {
			l[j] = f()
		}
		if i%7 == 0 { // a lone extreme anywhere in the block
			l[r.Intn(ThresholdBlock)*w+r.Intn(ThresholdBlock)] = uint8(r.Intn(2) * 255)
		}
		lo, hi, sum := blockStats(l, w)
		wlo, whi, wsum := referenceBlockStats(l, w)
		if lo != wlo || hi != whi || sum != wsum {
			t.Fatalf("block %v: blockStats = %d, %d, %d; want %d, %d, %d", l, lo, hi, sum, wlo, whi, wsum)
		}
	}
}

func BenchmarkHybridThresholds(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	l := make([]uint8, 4000*3000)
	for i := range l {
		l[i] = uint8(r.Intn(256))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HybridThresholds(l, 4000, 3000)
	}
}

// referenceRuns is ThresholdMap.runs up to v2.7, which classified one pixel
// at a time.
func referenceRuns(m *ThresholdMap, y int, inverted bool) []int {
	var dst []int
	src := m.l[y*m.W : (y+1)*m.W]
	t := m.t[m.block(y, m.H, m.bh)*m.bw:]
	prev, start := false, 0
	for x, v := range src {
		tb := t[0]
		if !m.global {
			tb = t[m.block(x, m.W, m.bw)]
		}
		if d := (v <= tb) != inverted; d != prev {
			dst = append(dst, x-start)
			prev, start = d, x
		}
	}
	return append(dst, m.W-start)
}

func TestRunsMatchReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	images := map[string]func(x, y int) uint8{
		"random": func(int, int) uint8 { return uint8(r.Intn(256)) },
		// Modules of a few pixels, so runs start and end inside words and
		// across them, and the thresholds vary between blocks.
		"modules": func(x, y int) uint8 { return uint8((x/3+y/5)%2*200 + r.Intn(40)) },
		"wide":    func(x, y int) uint8 { return uint8((x / 29) % 2 * 255) },
		"flat":    func(int, int) uint8 { return 128 },
		"bytes":   func(x, y int) uint8 { return uint8(x*37 + y) }, // every value, high bits
	}
	for name, f := range images {
		for _, w := range []int{40, 41, 47, 48, 49, 55, 56, 63, 64, 65, 83, 200, 203} {
			h := 41
			l := make([]uint8, w*h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					l[y*w+x] = f(x, y)
				}
			}
			tm := HybridThresholds(l, w, h)
			if tm.global {
				t.Fatalf("%s %dx%d: global threshold", name, w, h)
			}
			for y := 0; y < h; y++ {
				for _, inverted := range []bool{false, true} {
					got, want := tm.runs(y, inverted, nil), referenceRuns(tm, y, inverted)
					if !slices.Equal(got, want) {
						t.Fatalf("%s %dx%d row %d inverted %v: runs = %v, want %v", name, w, h, y, inverted, got, want)
					}
				}
			}
		}
	}
}

func BenchmarkRuns(b *testing.B) {
	for _, bc := range []struct {
		name string
		f    func(r *rand.Rand, x, y int) uint8
	}{
		// Noise is the worst case, with a class change at most pixels; the
		// modules of a code in a photo change every few pixels; texture
		// with gentle shading, most of a photo, rarely.
		{"noise", func(r *rand.Rand, _, _ int) uint8 { return uint8(r.Intn(256)) }},
		{"modules", func(r *rand.Rand, x, y int) uint8 { return uint8((x/4+y/4)%2*180 + r.Intn(40)) }},
		{"smooth", func(r *rand.Rand, x, y int) uint8 { return uint8(100 + (x+y)/64%50 + r.Intn(8)) }},
	} {
		r := rand.New(rand.NewSource(1))
		w, h := 4000, 3000
		l := make([]uint8, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				l[y*w+x] = bc.f(r, x, y)
			}
		}
		tm := HybridThresholds(l, w, h)
		b.Run(bc.name, func(b *testing.B) {
			var dst []int
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for y := 0; y < h; y += 3 {
					dst = tm.runs(y, false, dst[:0])
				}
			}
		})
	}
}

func TestLumaRange(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 1; n < 70; n++ {
		for k := 0; k < 50; k++ {
			l := make([]uint8, n)
			base := uint8(r.Intn(256))
			for i := range l {
				l[i] = base
			}
			// A few values anywhere, including the tail past the words.
			for j := r.Intn(3); j > 0; j-- {
				l[r.Intn(n)] = uint8(r.Intn(256))
			}
			wlo, whi := slices.Min(l), slices.Max(l)
			if lo, hi := LumaRange(l); lo != wlo || hi != whi {
				t.Fatalf("LumaRange(%v) = %d, %d; want %d, %d", l, lo, hi, wlo, whi)
			}
		}
	}
}

func TestFinderRowStep(t *testing.T) {
	for _, c := range []struct{ px, want int }{{640 * 480, 1}, {1920 * 1080, 2}, {4032 * 3024, 3}} {
		if got := FinderRowStep(c.px); got != c.want {
			t.Errorf("FinderRowStep(%d) = %d, want %d", c.px, got, c.want)
		}
	}
}
