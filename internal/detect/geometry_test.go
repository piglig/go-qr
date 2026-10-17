package detect

import (
	"math"
	"testing"
)

func TestQuadToQuad(t *testing.T) {
	src := [4][2]float64{{3.5, 3.5}, {25.5, 3.5}, {22.5, 22.5}, {3.5, 25.5}}
	dst := [4][2]float64{{40, 30}, {300, 60}, {270, 280}, {20, 250}}
	p, ok := QuadToQuad(src, dst)
	if !ok {
		t.Fatal("degenerate")
	}
	inv, ok := p.Inverse()
	if !ok {
		t.Fatal("not invertible")
	}
	for i := range src {
		x, y := p.Apply(src[i][0], src[i][1])
		if math.Hypot(x-dst[i][0], y-dst[i][1]) > 1e-9 {
			t.Errorf("corner %d maps to (%g, %g), want %v", i, x, y, dst[i])
		}
		u, v := inv.Apply(dst[i][0], dst[i][1])
		if math.Hypot(u-src[i][0], v-src[i][1]) > 1e-9 {
			t.Errorf("inverse of corner %d is (%g, %g), want %v", i, u, v, src[i])
		}
	}
	// A parallelogram gives an affine map.
	a, _ := squareToQuad([4][2]float64{{0, 0}, {2, 1}, {3, 4}, {1, 3}})
	if a.g != 0 || a.h != 0 {
		t.Errorf("parallelogram is not affine: %+v", a)
	}
	if _, ok := QuadToQuad(src, [4][2]float64{{0, 0}, {1, 1}, {2, 2}, {3, 3}}); ok {
		t.Error("collinear quadrilateral accepted")
	}
}

func TestFitLineIntersect(t *testing.T) {
	var a, b [][2]float64
	for i := 0; i < 10; i++ {
		x := float64(i)
		a = append(a, [2]float64{x, 2*x + 1 + 0.01*float64(i%2)}) // y = 2x+1
		b = append(b, [2]float64{x, -x + 7})                      // y = -x+7
	}
	la, _ := fitLine(a)
	lb, _ := fitLine(b)
	x, y, ok := la.intersect(lb)
	if !ok || math.Abs(x-2) > 0.02 || math.Abs(y-5) > 0.02 {
		t.Fatalf("intersection (%g, %g), want (2, 5)", x, y)
	}
	if _, _, ok := la.intersect(la); ok {
		t.Error("parallel lines intersect")
	}
	if _, ok := fitLine(a[:1]); ok {
		t.Error("line fitted to one point")
	}
}
