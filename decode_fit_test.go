package qr

import (
	"image"
	"image/color"
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

// warpBarrel renders src with radial barrel distortion about the image
// center: the pixel at normalized radius r shows the source at r·(1−k·r²).
func warpBarrel(src image.Image, k float64) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))
	cx, cy, norm := float64(w)/2, float64(h)/2, float64(w)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rx, ry := (float64(x)+0.5-cx)/norm, (float64(y)+0.5-cy)/norm
			f := 1 - k*(rx*rx+ry*ry)
			sx, sy := int(math.Floor(cx+rx*f*norm)), int(math.Floor(cy+ry*f*norm))
			c := color.Gray{Y: 255}
			if sx >= 0 && sy >= 0 && sx < w && sy < h {
				c = color.GrayModel.Convert(src.At(b.Min.X+sx, b.Min.Y+sy)).(color.Gray)
			}
			out.SetGray(x, y, c)
		}
	}
	return out
}

// TestDecodeLensDistortion covers the polynomial correction: barrel
// distortion bends a large symbol's grid enough at its corners that the
// homography alone does not decode it.
func TestDecodeLensDistortion(t *testing.T) {
	text := strings.Repeat("lens distortion ", 18)
	code := mustEncode(t, text, WithECC(ECCMedium))
	if code.Version() < 10 {
		t.Fatalf("version %d, want 10+", code.Version())
	}
	img := rotateGray(mustImage(t, code, WithScale(6)), 0.15)
	distorted := warpBarrel(img, -0.035)

	b := distorted.Bounds()
	w, h := b.Dx(), b.Dy()
	l := toLuma(distorted)
	tm := hybridThresholds(l, w, h)
	dark := tm.dark
	triples, err := findFinders(tm, false, dark)
	if err != nil {
		t.Fatal(err)
	}
	g, err := newSymbolGeometry(dark, triples[0])
	if err != nil {
		t.Fatal(err)
	}
	model, dim, _, ok := g.fitSymbol(dark)
	if !ok || dim != code.Size() {
		t.Fatalf("fit ok=%v dim=%d, want %d", ok, dim, code.Size())
	}
	if model.scale == 0 {
		t.Error("lens correction not applied")
	}
	if _, err := decodeGrid(readModules(l, w, h, model, dim, false)); err != nil {
		t.Errorf("corrected grid: %v", err)
	}
	if _, err := decodeGrid(readModules(l, w, h, model.h, dim, false)); err == nil {
		t.Error("the homography alone decodes: the test does not need the correction")
	}
	res, err := Decode(distorted)
	if err != nil || res.Text != text {
		t.Fatalf("Decode: %v", err)
	}
}

// TestDecodeMissingFinder covers structural completion: one finder pattern
// is painted over, as glare or damage would erase it, and the symbol is
// rotated so the fast path does not apply.
func TestDecodeMissingFinder(t *testing.T) {
	for _, n := range []int{20, 120, 300} {
		text := strings.Repeat("c", n)
		code := mustEncode(t, text, WithECC(ECCHigh))
		const scale, qz = 6, 8
		for corner, at := range map[string][2]int{"top-left": {0, 0}, "top-right": {code.Size() - 7, 0}, "bottom-left": {0, code.Size() - 7}} {
			img := mustImage(t, code, WithScale(scale), WithQuietZone(qz))
			// Paint the finder and its separator mid-gray.
			x0, y0 := (qz+at[0]-1)*scale, (qz+at[1]-1)*scale
			for y := y0; y < y0+9*scale; y++ {
				for x := x0; x < x0+9*scale; x++ {
					img.Set(x, y, color.Gray{Y: 128})
				}
			}
			res, err := Decode(rotateGray(img, 0.2))
			if err != nil || res.Text != text {
				t.Errorf("version %d, %s finder missing: %v", code.Version(), corner, err)
			}
		}
	}
}

// TestPerspectiveFallback covers the fallback for finders whose corners
// cannot be measured: transforms anchored on the alignment pattern, the
// finder edges or the parallelogram of the finder centers.
func TestPerspectiveFallback(t *testing.T) {
	for _, n := range []int{11, 120} {
		text := strings.Repeat("f", n)
		code := mustEncode(t, text)
		img := mustImage(t, code, WithScale(8))
		w := float64(img.Bounds().Dx())
		q := [4][2]float64{{0.1 * w, 0.12 * w}, {0.9 * w, 0.05 * w}, {0.95 * w, 0.95 * w}, {0.05 * w, 0.88 * w}}
		warped := warpPerspective(img, q, int(w), int(w))
		b := warped.Bounds()
		l := toLuma(warped)
		tm := hybridThresholds(l, b.Dx(), b.Dy())
		triples, err := findFinders(tm, false, tm.dark)
		if err != nil {
			t.Fatal(err)
		}
		g, err := newSymbolGeometry(tm.dark, triples[0])
		if err != nil {
			t.Fatal(err)
		}
		ps := g.transforms(tm.dark)
		if code.Version() > 1 && len(ps) != 2 {
			t.Errorf("version %d: %d transforms, want the alignment pattern and the edges", code.Version(), len(ps))
		}
		decoded := false
		for _, p := range ps {
			if res, err := decodeGrid(readModules(l, b.Dx(), b.Dy(), p, g.dim, false)); err == nil && res.Text == text {
				decoded = true
			}
		}
		if !decoded {
			t.Errorf("version %d: no fallback transform decodes", code.Version())
		}
	}
}
