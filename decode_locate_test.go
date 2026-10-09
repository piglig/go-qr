package qr

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// TestFinderIndexMatchesLinear merges random hits, clustered so that many
// merge and some move between cells, through finderIndex and through the
// linear search it replaces, and expects the same candidates.
func TestFinderIndexMatchesLinear(t *testing.T) {
	const w, h = 300, 200
	r := rand.New(rand.NewSource(1))
	for _, n := range []int{10, finderIndexMin + 1, 5000} {
		idx := finderIndex{w: w, h: h}
		var want []finderPattern
		for k := 0; k < n; k++ {
			// Some hits fall outside the image, as refined centers can.
			cx := r.Float64()*(w+40) - 20
			cy := r.Float64()*(h+40) - 20
			if k > 0 && r.Intn(2) == 0 {
				c := want[r.Intn(len(want))]
				cx, cy = c.x+r.NormFloat64()*2, c.y+r.NormFloat64()*2
			}
			module := 0.5 + r.Float64()*float64(r.Intn(3)*20+3)

			merged := false
			for i := range want {
				if math.Abs(want[i].x-cx) < module && math.Abs(want[i].y-cy) < module {
					nn := float64(want[i].count)
					want[i].x = (want[i].x*nn + cx) / (nn + 1)
					want[i].y = (want[i].y*nn + cy) / (nn + 1)
					want[i].moduleSize = (want[i].moduleSize*nn + module) / (nn + 1)
					want[i].count++
					merged = true
					break
				}
			}
			if !merged {
				want = append(want, finderPattern{x: cx, y: cy, moduleSize: module, count: 1})
			}

			if i := idx.find(cx, cy, module); i >= 0 {
				c := idx.cands[i]
				nn := float64(c.count)
				c.x = (c.x*nn + cx) / (nn + 1)
				c.y = (c.y*nn + cy) / (nn + 1)
				c.moduleSize = (c.moduleSize*nn + module) / (nn + 1)
				c.count++
				idx.set(i, c)
			} else {
				idx.append(finderPattern{x: cx, y: cy, moduleSize: module, count: 1})
			}
		}
		assertEqual(t, want, idx.cands, "%d hits", n)
		if len(want) > finderIndexMin {
			assertNotNil(t, idx.head, "%d candidates should be bucketed", len(want))
		}
	}
}

// TestDecodeFinderTiles decodes an image tiled with finder patterns, whose
// candidates the finder scan used to merge in time quadratic in their
// number: on a fast desktop it took 10 s, and takes 70 ms. The bound leaves
// room for slow machines.
func TestDecodeFinderTiles(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test")
	}
	img := finderTiles(3200, 3200, 1)
	start := time.Now()
	if _, err := Decode(img); err == nil {
		t.Fatal("decoded an image without a code")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Decode took %v", d)
	}
}
