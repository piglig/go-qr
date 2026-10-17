package detect

import (
	"math/rand"
	"testing"
)

// referenceRangePyramid is newRangePyramid up to v2.6, which allocated
// each level's slices separately.
func referenceRangePyramid(bw, bh int, at func(i int) (lo, hi uint8)) *rangePyramid {
	lo, hi := make([]uint8, bw*bh), make([]uint8, bw*bh)
	for i := range lo {
		lo[i], hi[i] = at(i)
	}
	p := &rangePyramid{}
	w, h := bw, bh
	for w > 1 || h > 1 {
		nw, nh := (w+1)/2, (h+1)/2
		clo, chi := make([]uint8, nw*nh), make([]uint8, nw*nh)
		for y := 0; y < nh; y++ {
			for x := 0; x < nw; x++ {
				l, u := uint8(255), uint8(0)
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						if yy, xx := 2*y+dy, 2*x+dx; yy < h && xx < w {
							l, u = min(l, lo[yy*w+xx]), max(u, hi[yy*w+xx])
						}
					}
				}
				clo[y*nw+x], chi[y*nw+x] = l, u
			}
		}
		// Window ranges over 3×3 cells, separably.
		tlo, thi := make([]uint8, nw*nh), make([]uint8, nw*nh)
		for y := 0; y < nh; y++ {
			for x := 0; x < nw; x++ {
				l, u := uint8(255), uint8(0)
				for xx := max(0, x-1); xx <= min(nw-1, x+1); xx++ {
					l, u = min(l, clo[y*nw+xx]), max(u, chi[y*nw+xx])
				}
				tlo[y*nw+x], thi[y*nw+x] = l, u
			}
		}
		wlo, whi := make([]uint8, nw*nh), make([]uint8, nw*nh)
		for y := 0; y < nh; y++ {
			for x := 0; x < nw; x++ {
				l, u := uint8(255), uint8(0)
				for yy := max(0, y-1); yy <= min(nh-1, y+1); yy++ {
					l, u = min(l, tlo[yy*nw+x]), max(u, thi[yy*nw+x])
				}
				wlo[y*nw+x], whi[y*nw+x] = l, u
			}
		}
		p.levels = append(p.levels, rangeLevel{wlo, whi, nw, nh})
		lo, hi, w, h = clo, chi, nw, nh
	}
	return p
}

func TestRangePyramidMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, sz := range [][2]int{{1, 1}, {1, 7}, {2, 2}, {3, 5}, {5, 3}, {17, 9}, {64, 64}, {75, 100}, {504, 378}} {
		bw, bh := sz[0], sz[1]
		lo, hi := make([]uint8, bw*bh), make([]uint8, bw*bh)
		for i := range lo {
			a, b := uint8(rng.Intn(256)), uint8(rng.Intn(256))
			lo[i], hi[i] = min(a, b), max(a, b)
		}
		at := func(i int) (uint8, uint8) { return lo[i], hi[i] }
		got, want := newRangePyramid(bw, bh, at), referenceRangePyramid(bw, bh, at)
		if len(got.levels) != len(want.levels) {
			t.Fatalf("%dx%d: %d levels, want %d", bw, bh, len(got.levels), len(want.levels))
		}
		for k, g := range got.levels {
			w := want.levels[k]
			if g.w != w.w || g.h != w.h || string(g.lo) != string(w.lo) || string(g.hi) != string(w.hi) {
				t.Fatalf("%dx%d: level %d differs", bw, bh, k)
			}
		}
	}
}
