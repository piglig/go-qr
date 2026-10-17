package detect

import (
	"math"
	"math/rand"
	"testing"
)

func TestFitHomography(t *testing.T) {
	want, _ := QuadToQuad([4][2]float64{{0, 0}, {65, 0}, {65, 65}, {0, 65}}, [4][2]float64{{40, 30}, {520, 80}, {480, 560}, {20, 500}})
	r := rand.New(rand.NewSource(1))
	var cs []correspondence
	for i := 0; i < 40; i++ {
		u, v := r.Float64()*65, r.Float64()*65
		x, y := want.Apply(u, v)
		cs = append(cs, correspondence{u, v, x + r.NormFloat64()*0.2, y + r.NormFloat64()*0.2, 1})
	}
	got, ok := fitHomography(cs)
	if !ok {
		t.Fatal("fit failed")
	}
	for _, p := range [][2]float64{{0, 0}, {65, 65}, {32.5, 10}, {60, 3}} {
		gx, gy := got.Apply(p[0], p[1])
		wx, wy := want.Apply(p[0], p[1])
		if math.Hypot(gx-wx, gy-wy) > 0.5 {
			t.Errorf("at %v: got (%.2f, %.2f), want (%.2f, %.2f)", p, gx, gy, wx, wy)
		}
	}
	if _, ok := fitHomography(cs[:3]); ok {
		t.Error("fit from three points")
	}
}

func TestRayEdges(t *testing.T) {
	// Dark for 10 <= x < 20, light elsewhere.
	dark := func(x, y int) bool { return x >= 10 && x < 20 }
	e, n := rayEdges(dark, 12.3, 5.7, 1, 0, 30, 3)
	if n != 1 || math.Abs(e[0][0]-20) > 1e-9 || math.Abs(e[0][1]-5.7) > 1e-9 {
		t.Fatalf("got %d crossings %v, want one at (20, 5.7)", n, e[:n])
	}
	// A slanted ray crosses exactly on the pixel boundary x = 10.
	dx, dy := -math.Cos(0.4), math.Sin(0.4)
	e, n = rayEdges(dark, 15, 3, dx, dy, 30, 1)
	if n != 1 || math.Abs(e[0][0]-10) > 1e-9 {
		t.Fatalf("slanted: got %v", e[:n])
	}
	if _, n := rayEdges(dark, 15, 3, 1, 0, 2, 1); n != 0 {
		t.Error("crossing found beyond maxLen")
	}
}

func TestAlignmentWarpInterpolates(t *testing.T) {
	// Version 7: alignment rows and columns at modules 6, 22 and 38.
	w := alignmentWarp{n: 3, nodes: [7]float64{6.5, 22.5, 38.5}}
	w.dx[1*3+1], w.dy[1*3+1] = 2, -4 // the center pattern only
	for _, tc := range []struct{ u, v, dx, dy float64 }{
		{22.5, 22.5, 2, -4},   // at the node
		{14.5, 22.5, 1, -2},   // halfway to the left column
		{14.5, 14.5, 0.5, -1}, // halfway to a corner
		{6.5, 38.5, 0, 0},     // a finder node
		{0, 22.5, 0, 0},       // past the left column: held at its value
		{44, 22.5, 0, 0},
		{22.5, 44, 0, 0},
	} {
		dx, dy := w.at(tc.u, tc.v)
		if math.Abs(dx-tc.dx) > 1e-9 || math.Abs(dy-tc.dy) > 1e-9 {
			t.Errorf("at(%v, %v) = (%v, %v), want (%v, %v)", tc.u, tc.v, dx, dy, tc.dx, tc.dy)
		}
	}
	m := GridModel{H: Perspective{1, 0, 0, 0, 1, 0, 0, 0}}
	if x, y := m.Apply(10, 20); x != 10 || y != 20 {
		t.Errorf("model without a warp maps (10, 20) to (%v, %v)", x, y)
	}
	m.warp = w
	if x, y := m.Apply(22.5, 22.5); x != 24.5 || y != 18.5 {
		t.Errorf("warped model maps (22.5, 22.5) to (%v, %v), want (24.5, 18.5)", x, y)
	}
}
