package qr

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestQuadToQuad(t *testing.T) {
	src := [4][2]float64{{3.5, 3.5}, {25.5, 3.5}, {22.5, 22.5}, {3.5, 25.5}}
	dst := [4][2]float64{{40, 30}, {300, 60}, {270, 280}, {20, 250}}
	p, ok := quadToQuad(src, dst)
	if !ok {
		t.Fatal("degenerate")
	}
	inv, ok := p.inverse()
	if !ok {
		t.Fatal("not invertible")
	}
	for i := range src {
		x, y := p.apply(src[i][0], src[i][1])
		if math.Hypot(x-dst[i][0], y-dst[i][1]) > 1e-9 {
			t.Errorf("corner %d maps to (%g, %g), want %v", i, x, y, dst[i])
		}
		u, v := inv.apply(dst[i][0], dst[i][1])
		if math.Hypot(u-src[i][0], v-src[i][1]) > 1e-9 {
			t.Errorf("inverse of corner %d is (%g, %g), want %v", i, u, v, src[i])
		}
	}
	// A parallelogram gives an affine map.
	a, _ := squareToQuad([4][2]float64{{0, 0}, {2, 1}, {3, 4}, {1, 3}})
	if a.g != 0 || a.h != 0 {
		t.Errorf("parallelogram is not affine: %+v", a)
	}
	if _, ok := quadToQuad(src, [4][2]float64{{0, 0}, {1, 1}, {2, 2}, {3, 3}}); ok {
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

// warpPerspective renders src as seen through the homography that maps its
// corners to dst (in the order top-left, top-right, bottom-right,
// bottom-left), on a white w×h canvas, with bilinear sampling.
func warpPerspective(src image.Image, dst [4][2]float64, w, h int) *image.Gray {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	p, ok := quadToQuad([4][2]float64{{0, 0}, {sw, 0}, {sw, sh}, {0, sh}}, dst)
	if !ok {
		panic("degenerate warp")
	}
	inv, _ := p.inverse()
	gray := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			gray.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= b.Dx() || y >= b.Dy() {
			return 255
		}
		return float64(gray.Pix[y*gray.Stride+x])
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u, v := inv.apply(float64(x)+0.5, float64(y)+0.5)
			u, v = u-0.5, v-0.5
			x0, y0 := int(math.Floor(u)), int(math.Floor(v))
			fx, fy := u-float64(x0), v-float64(y0)
			c := (at(x0, y0)*(1-fx)+at(x0+1, y0)*fx)*(1-fy) + (at(x0, y0+1)*(1-fx)+at(x0+1, y0+1)*fx)*fy
			out.SetGray(x, y, color.Gray{Y: uint8(c + 0.5)})
		}
	}
	return out
}

func TestDecodePerspective(t *testing.T) {
	// Each quadrilateral is a symbol seen at roughly 30 to 40 degrees, in
	// several directions: the far side is about 30% shorter.
	quads := map[string][4][2]float64{
		"left far":   {{40, 60}, {380, 20}, {380, 400}, {40, 360}},
		"top far":    {{70, 40}, {350, 40}, {400, 380}, {20, 380}},
		"corner far": {{60, 60}, {360, 30}, {390, 390}, {30, 360}},
		"rotated":    {{200, 20}, {390, 230}, {200, 400}, {30, 210}},
	}
	for _, n := range []int{11, 30, 120, 300} { // versions 1, 2-3, 7ish, 12ish
		text := strings.Repeat("p", n)
		code := mustEncode(t, text, WithECC(ECCMedium))
		img := mustImage(t, code, WithScale(8))
		for name, q := range quads {
			warped := warpPerspective(img, q, 420, 420)
			res, err := Decode(warped)
			if err != nil {
				t.Errorf("version %d, %s: %v", code.Version(), name, err)
				continue
			}
			assertEqual(t, text, res.Text)
		}
	}
}

// TestDecodeRotated45 covers module sizes measured along the image axes,
// which are √2 too large at 45 degrees and made the robust path misjudge
// the version, or reject a version 1 symbol's finders as too close.
func TestDecodeRotated45(t *testing.T) {
	for _, n := range []int{11, 60, 200} {
		text := strings.Repeat("r", n)
		code := mustEncode(t, text)
		img := mustImage(t, code, WithScale(6), WithQuietZone(16)) // room for the rotated corners
		for _, deg := range []float64{30, 45, 60, 135} {
			res, err := Decode(rotateGray(img, deg*math.Pi/180))
			if err != nil || res.Text != text {
				t.Errorf("version %d at %v degrees: %v", code.Version(), deg, err)
			}
		}
	}
}
