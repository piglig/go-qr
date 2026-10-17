package detect

import (
	"math"
	"math/rand"
	"testing"
)

// TestFinderIndexMatchesLinear merges random hits, clustered so that many
// merge and some move between cells, through finderIndex and through the
// linear search it replaces, and expects the same candidates.
func TestFinderIndexMatchesLinear(t *testing.T) {
	const w, h = 300, 200
	r := rand.New(rand.NewSource(1))
	for _, n := range []int{10, finderIndexMin + 1, 5000} {
		idx := finderIndex{w: w, h: h}
		var want []FinderPattern
		for k := 0; k < n; k++ {
			// Some hits fall outside the image, as refined centers can.
			cx := r.Float64()*(w+40) - 20
			cy := r.Float64()*(h+40) - 20
			if k > 0 && r.Intn(2) == 0 {
				c := want[r.Intn(len(want))]
				cx, cy = c.X+r.NormFloat64()*2, c.Y+r.NormFloat64()*2
			}
			module := 0.5 + r.Float64()*float64(r.Intn(3)*20+3)

			merged := false
			for i := range want {
				if math.Abs(want[i].X-cx) < module && math.Abs(want[i].Y-cy) < module {
					nn := float64(want[i].Count)
					want[i].X = (want[i].X*nn + cx) / (nn + 1)
					want[i].Y = (want[i].Y*nn + cy) / (nn + 1)
					want[i].ModuleSize = (want[i].ModuleSize*nn + module) / (nn + 1)
					want[i].Count++
					merged = true
					break
				}
			}
			if !merged {
				want = append(want, FinderPattern{X: cx, Y: cy, ModuleSize: module, Count: 1})
			}

			if i := idx.find(cx, cy, module); i >= 0 {
				c := idx.cands[i]
				nn := float64(c.Count)
				c.X = (c.X*nn + cx) / (nn + 1)
				c.Y = (c.Y*nn + cy) / (nn + 1)
				c.ModuleSize = (c.ModuleSize*nn + module) / (nn + 1)
				c.Count++
				idx.set(i, c)
			} else {
				idx.append(FinderPattern{X: cx, Y: cy, ModuleSize: module, Count: 1})
			}
		}
		assertEqual(t, want, idx.cands, "%d hits", n)
		if len(want) > finderIndexMin {
			assertNotNil(t, idx.head, "%d candidates should be bucketed", len(want))
		}
	}
}
