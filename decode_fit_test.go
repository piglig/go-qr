package qr

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

func TestFitHomography(t *testing.T) {
	want, _ := quadToQuad([4][2]float64{{0, 0}, {65, 0}, {65, 65}, {0, 65}}, [4][2]float64{{40, 30}, {520, 80}, {480, 560}, {20, 500}})
	r := rand.New(rand.NewSource(1))
	var cs []correspondence
	for i := 0; i < 40; i++ {
		u, v := r.Float64()*65, r.Float64()*65
		x, y := want.apply(u, v)
		cs = append(cs, correspondence{u, v, x + r.NormFloat64()*0.2, y + r.NormFloat64()*0.2, 1})
	}
	got, ok := fitHomography(cs)
	if !ok {
		t.Fatal("fit failed")
	}
	for _, p := range [][2]float64{{0, 0}, {65, 65}, {32.5, 10}, {60, 3}} {
		gx, gy := got.apply(p[0], p[1])
		wx, wy := want.apply(p[0], p[1])
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

// TestDecodeSteepPerspective covers symbols seen at about 50 degrees from a
// camera close to them: the far side is under two thirds the length of the
// near one, finders appear much narrower than tall, and the triangle of
// finder centers is far from right isosceles.
func TestDecodeSteepPerspective(t *testing.T) {
	quads := map[string][4][2]float64{
		"left far":   {{30, 100}, {470, 20}, {470, 500}, {30, 420}},
		"bottom far": {{20, 30}, {500, 30}, {420, 420}, {100, 420}},
		"corner far": {{20, 20}, {480, 70}, {400, 400}, {70, 480}},
	}
	for _, n := range []int{11, 120, 300} {
		text := strings.Repeat("t", n)
		code := mustEncode(t, text, WithECC(ECCMedium))
		img := mustImage(t, code, WithScale(10))
		for name, q := range quads {
			res, err := Decode(warpPerspective(img, q, 520, 520))
			if err != nil || res.Text != text {
				t.Errorf("version %d, %s: %v", code.Version(), name, err)
			}
		}
	}
}

// TestDecodeStyledGeometry covers finder and module styles through rotation
// and perspective. Round finders have no corners to measure, so they take
// the fallback geometry.
func TestDecodeStyledGeometry(t *testing.T) {
	for _, fs := range []FinderShape{FinderSquare, FinderRounded, FinderCircle} {
		for _, ms := range []ModuleShape{ModuleSquare, ModuleDot, ModuleRounded} {
			for _, n := range []int{20, 150} {
				text := strings.Repeat("s", n)
				code := mustEncode(t, text, WithECC(ECCHigh))
				img := mustImage(t, code, WithScale(8), WithQuietZone(12), WithFinderShape(fs), WithModuleShape(ms))
				if res, err := Decode(rotateGray(img, 25*math.Pi/180)); err != nil || res.Text != text {
					t.Errorf("finder %v, modules %v, version %d, rotated: %v", fs, ms, code.Version(), err)
				}
				w := float64(img.Bounds().Dx())
				q := [4][2]float64{{0.1 * w, 0.12 * w}, {0.9 * w, 0.05 * w}, {0.95 * w, 0.95 * w}, {0.05 * w, 0.88 * w}}
				if res, err := Decode(warpPerspective(img, q, int(w), int(w))); err != nil || res.Text != text {
					t.Errorf("finder %v, modules %v, version %d, perspective: %v", fs, ms, code.Version(), err)
				}
			}
		}
	}
}
