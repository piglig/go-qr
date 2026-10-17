package qr

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/piglig/go-qr/v2/internal/detect"
)

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
// and detect.Perspective. Round finders have no corners to measure, so they take
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
					t.Errorf("finder %v, modules %v, version %d, detect.Perspective: %v", fs, ms, code.Version(), err)
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
	l := detect.ToLuma(distorted)
	tm := detect.HybridThresholds(l, w, h)
	dark := tm.Dark
	triples, err := detect.FindFinders(tm, false, dark)
	if err != nil {
		t.Fatal(err)
	}
	g, err := detect.NewSymbolGeometry(dark, triples[0])
	if err != nil {
		t.Fatal(err)
	}
	model, dim, _, ok := g.FitSymbol(dark)
	if !ok || dim != code.Size() {
		t.Fatalf("fit ok=%v dim=%d, want %d", ok, dim, code.Size())
	}
	if model.Scale == 0 {
		t.Error("lens correction not applied")
	}
	if _, err := decodeGrid(detect.ReadModules(l, w, h, model, dim, false), nil); err != nil {
		t.Errorf("corrected grid: %v", err)
	}
	if _, err := decodeGrid(detect.ReadModules(l, w, h, model.H, dim, false), nil); err == nil {
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
		l := detect.ToLuma(warped)
		tm := detect.HybridThresholds(l, b.Dx(), b.Dy())
		triples, err := detect.FindFinders(tm, false, tm.Dark)
		if err != nil {
			t.Fatal(err)
		}
		g, err := detect.NewSymbolGeometry(tm.Dark, triples[0])
		if err != nil {
			t.Fatal(err)
		}
		ps := g.Transforms(tm.Dark)
		if code.Version() > 1 && len(ps) != 2 {
			t.Errorf("version %d: %d transforms, want the alignment pattern and the edges", code.Version(), len(ps))
		}
		decoded := false
		for _, p := range ps {
			if res, err := decodeGrid(detect.ReadModules(l, b.Dx(), b.Dy(), p, g.Dim, false), nil); err == nil && res.Text == text {
				decoded = true
			}
		}
		if !decoded {
			t.Errorf("version %d: no fallback transform decodes", code.Version())
		}
	}
}

// bulge renders code and displaces its pixels by a smooth bump of amp
// pixels, at most, around (cx, cy), as a sheet that does not lie flat
// does; no homography follows it.
func bulge(code *Code, scale, quiet int, amp, cx, cy, radius float64) *image.Gray {
	src, _ := code.Image(WithScale(scale), WithQuietZone(quiet))
	b := src.Bounds()
	out := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			g := amp * math.Exp(-(dx*dx+dy*dy)/(2*radius*radius))
			sx, sy := int(math.Round(float64(x)-g)), int(math.Round(float64(y)-0.6*g))
			v := uint8(255)
			if image.Pt(sx, sy).In(b) {
				v = src.RGBAAt(sx, sy).G
			}
			out.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return out
}

func TestDecodeLocalBulge(t *testing.T) {
	// A version 23 code at level L, whose global fit leaves too many
	// errors in the displaced region without the local correction.
	text := strings.Repeat("local distortion ", 60)
	code, err := Encode(text, WithECC(ECCLow))
	assertNoError(t, err)
	const scale, quiet = 5, 4
	side := float64((code.Size() + 2*quiet) * scale)
	for _, amp := range []float64{2, 2.4, 3} { // in modules
		img := bulge(code, scale, quiet, amp*scale, side*0.62, side*0.58, side*0.2)
		res, err := Decode(img)
		if err != nil {
			t.Errorf("bulge of %v modules: %v", amp, err)
			continue
		}
		assertEqual(t, text, res.Text)
	}
}

// warpPerspective renders src as seen through the homography that maps its
// corners to dst (in the order top-left, top-right, bottom-right,
// bottom-left), on a white w×h canvas, with bilinear sampling.
func warpPerspective(src image.Image, dst [4][2]float64, w, h int) *image.Gray {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	p, ok := detect.QuadToQuad([4][2]float64{{0, 0}, {sw, 0}, {sw, sh}, {0, sh}}, dst)
	if !ok {
		panic("degenerate warp")
	}
	inv, _ := p.Inverse()
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
			u, v := inv.Apply(float64(x)+0.5, float64(y)+0.5)
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

func TestThresholdLargeModules(t *testing.T) {
	// Inside a module 40 pixels wide every 8×8 block is uniform, and so is
	// the 5×5-block window around most of them; the range detect.Pyramid must
	// still classify the module's interior by the contrast around it.
	code := mustEncode(t, "large modules")
	img := mustImage(t, code, WithScale(40), WithQuietZone(2))
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	l := detect.ToLuma(img)
	tm := detect.HybridThresholds(l, w, h)
	wrong := 0
	for y := 0; y < h; y += 7 {
		for x := 0; x < w; x += 7 {
			if tm.Dark(x, y) != (l[y*w+x] < 128) {
				wrong++
			}
		}
	}
	if total := (w / 7) * (h / 7); wrong*100 > total {
		t.Errorf("%d of %d sampled pixels misclassified", wrong, total)
	}
}

// screenPhoto renders a symbol as a photographed screen: large modules
// overlaid with a fine pixel grid, which breaks the finder patterns' runs at
// full resolution, slightly rotated, on a gray surround.
func screenPhoto(t *testing.T, text string) *image.Gray {
	code := mustEncode(t, text)
	sym := rotateGray(mustImage(t, code, WithScale(36), WithQuietZone(6)), 0.05)
	b := sym.Bounds()
	out := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			v := float64(sym.GrayAt(x, y).Y)
			if x%3 == 0 || y%3 == 0 { // the gaps between screen pixels
				v *= 0.55
			}
			out.SetGray(x, y, color.Gray{Y: uint8(30 + v*0.75)})
		}
	}
	return out
}

func TestDecodeScreenPhoto(t *testing.T) {
	text := "https://example.com/screen"
	if res, err := Decode(screenPhoto(t, text)); err != nil || res.Text != text {
		t.Fatalf("Decode: %v", err)
	}
}

func TestDecodeSmallCodeInLargePhoto(t *testing.T) {
	// A 4-megapixel image is scanned every third row; a symbol of 3-pixel
	// modules must still be found.
	text := "small code, large photo"
	code := mustEncode(t, text)
	sym := rotateGray(mustImage(t, code, WithScale(3), WithQuietZone(6)), 0.1)
	const w, h = 2400, 1800
	img := image.NewGray(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(2))
	for i := range img.Pix {
		img.Pix[i] = uint8(150 + r.Intn(60))
	}
	sb := sym.Bounds()
	for y := 0; y < sb.Dy(); y++ {
		copy(img.Pix[(1200+y)*w+700:], sym.Pix[y*sym.Stride:y*sym.Stride+sb.Dx()])
	}
	if detect.FinderRowStep(w*h) != 3 {
		t.Fatal("test image is not scanned every third row")
	}
	if res, err := Decode(img); err != nil || res.Text != text {
		t.Fatalf("Decode: %v", err)
	}
}

func TestReadModulesUnevenLight(t *testing.T) {
	// Module-space thresholding against a window of modules follows a
	// lighting gradient that spans the symbol's whole contrast.
	code := mustEncode(t, strings.Repeat("u", 60))
	img := mustImage(t, code, WithScale(5), WithQuietZone(0))
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	l := detect.ToLuma(img)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			f := 0.25 + 0.75*float64(x)/float64(w) // dark on the left
			l[y*w+x] = uint8(math.Round(float64(l[y*w+x]) * f))
		}
	}
	d := float64(code.Size())
	p, _ := detect.QuadToQuad([4][2]float64{{0, 0}, {d, 0}, {d, d}, {0, d}}, [4][2]float64{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}})
	if _, err := decodeModules(detect.ReadModules(l, w, h, p, code.Size(), false), nil); err != nil {
		t.Fatalf("read under a lighting gradient: %v", err)
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
