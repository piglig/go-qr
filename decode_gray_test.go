package qr

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"strings"
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
	tm := hybridThresholds(l, w, h)
	for y := 0; y < h; y++ {
		// runs must agree with dark pixel by pixel.
		runs := tm.runs(y, false, nil)
		x, dark := 0, false
		for i, n := range runs {
			if i > 0 && n == 0 {
				t.Fatalf("row %d: empty run %d in %v", y, i, runs)
			}
			for k := 0; k < n; k++ {
				if got := tm.dark(x, y); got != dark {
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
	if tm.dark(-1, 0) || tm.dark(0, -1) || tm.dark(w, 0) || tm.dark(0, h) {
		t.Error("pixels outside the image are dark")
	}
	inv := tm.runs(0, true, nil)
	if inv[0] != 0 && tm.dark(0, 0) == false {
		t.Errorf("inverted row starts with a light run %d at a light pixel", inv[0])
	}
}

func TestThresholdLargeModules(t *testing.T) {
	// Inside a module 40 pixels wide every 8×8 block is uniform, and so is
	// the 5×5-block window around most of them; the range pyramid must
	// still classify the module's interior by the contrast around it.
	code := mustEncode(t, "large modules")
	img := mustImage(t, code, WithScale(40), WithQuietZone(2))
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	l := toLuma(img)
	tm := hybridThresholds(l, w, h)
	wrong := 0
	for y := 0; y < h; y += 7 {
		for x := 0; x < w; x += 7 {
			if tm.dark(x, y) != (l[y*w+x] < 128) {
				wrong++
			}
		}
	}
	if total := (w / 7) * (h / 7); wrong*100 > total {
		t.Errorf("%d of %d sampled pixels misclassified", wrong, total)
	}
}

func TestRangePyramid(t *testing.T) {
	// A 10×10-block image, flat except for one block of contrast at (8, 8).
	lo := make([]int, 100)
	hi := make([]int, 100)
	for i := range lo {
		lo[i], hi[i] = 200, 200
	}
	lo[88], hi[88] = 10, 240
	p := newRangePyramid(10, 10, func(i int) (int, int) { return lo[i], hi[i] })
	if _, _, ok := p.contrast(1, 1, 24); !ok {
		t.Fatal("no contrast found from the far corner")
	}
	l, h, _ := p.contrast(8, 8, 24)
	if l != 10 || h != 240 {
		t.Errorf("contrast next to the block = %d..%d, want 10..240", l, h)
	}
	flat := newRangePyramid(4, 4, func(int) (int, int) { return 100, 110 })
	if _, _, ok := flat.contrast(0, 0, 24); ok {
		t.Error("contrast found in a flat image")
	}
}

func TestHalve(t *testing.T) {
	l := []uint8{0, 4, 8, 12, 100, 104, 108, 112, 255, 255}
	got, w, h := halve(l, 5, 2)
	if w != 2 || h != 1 || len(got) != 2 {
		t.Fatalf("halve: %dx%d %v", w, h, got)
	}
	if got[0] != (0+4+104+108+2)/4 || got[1] != (8+12+112+255+2)/4 {
		t.Errorf("halve = %v", got)
	}
}

func TestFinderRowStep(t *testing.T) {
	for _, c := range []struct{ px, want int }{{640 * 480, 1}, {1920 * 1080, 2}, {4032 * 3024, 3}} {
		if got := finderRowStep(c.px); got != c.want {
			t.Errorf("finderRowStep(%d) = %d, want %d", c.px, got, c.want)
		}
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
	if finderRowStep(w*h) != 3 {
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
	l := toLuma(img)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			f := 0.25 + 0.75*float64(x)/float64(w) // dark on the left
			l[y*w+x] = uint8(math.Round(float64(l[y*w+x]) * f))
		}
	}
	d := float64(code.Size())
	p, _ := quadToQuad([4][2]float64{{0, 0}, {d, 0}, {d, d}, {0, d}}, [4][2]float64{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}})
	if _, err := decodeModules(readModules(l, w, h, p, code.Size(), false)); err != nil {
		t.Fatalf("read under a lighting gradient: %v", err)
	}
}
