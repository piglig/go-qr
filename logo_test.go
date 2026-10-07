package qr

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"testing"
)

func makeTestLogo(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

var red = color.RGBA{R: 0xff, A: 0xff}

func TestLogoImageIsCentered(t *testing.T) {
	code, err := Encode("Hello, world!", WithECC(ECCHigh))
	assertNoError(t, err)

	const scale, qz = 4, 3
	img, err := code.Image(WithScale(scale), WithQuietZone(qz), WithLogo(makeTestLogo(10, 10, red), 0.2))
	assertNoError(t, err)

	// The logo is centered on the symbol, so its center pixel is red and the
	// pad around it has the background color.
	side := img.Bounds().Dx()
	assertTrue(t, sameColor(img.At(side/2, side/2), red), "center is not the logo")

	box, inner := (&logoConfig{ratio: 0.2}).rects(code.Size(), &renderConfig{scale: scale, quietZone: qz})
	assertEqual(t, side-box.Max.X, box.Min.X, "logo box is not horizontally centered")
	assertEqual(t, side-box.Max.Y, box.Min.Y, "logo box is not vertically centered")
	assertTrue(t, sameColor(img.At(box.Min.X, box.Min.Y), color.White), "pad is not the background color")
	assertTrue(t, sameColor(img.At(inner.Min.X, inner.Min.Y), red), "inner area is not the logo")
}

func TestLogoPadUsesBackground(t *testing.T) {
	code, _ := Encode("Hello, world!", WithECC(ECCHigh))
	bg := color.RGBA{R: 0x20, G: 0x40, B: 0x60, A: 0xff}
	img, err := code.Image(WithBackground(bg), WithForeground(color.White), WithLogo(makeTestLogo(4, 4, red), 0.2))
	assertNoError(t, err)
	box, _ := (&logoConfig{ratio: 0.2}).rects(code.Size(), &renderConfig{scale: defaultScale, quietZone: defaultQuietZone})
	assertTrue(t, sameColor(img.At(box.Min.X, box.Min.Y), bg))
}

func TestLogoPNGUsesFullColor(t *testing.T) {
	code, _ := Encode("Hello, world!", WithECC(ECCHigh))
	data, err := code.PNG(WithLogo(makeTestLogo(8, 8, red), 0.2))
	assertNoError(t, err)
	img, err := png.Decode(bytes.NewReader(data))
	assertNoError(t, err)
	side := img.Bounds().Dx()
	assertTrue(t, sameColor(img.At(side/2, side/2), red))
}

func TestLogoSVGIsCentered(t *testing.T) {
	code, _ := Encode("hi", WithECC(ECCHigh))
	// Version 1 is 21 modules; the 0.2 logo is 3 modules plus padding = 5,
	// starting at module 8. With quiet zone 4 and scale 10 that is 120 units.
	data, err := code.SVG(WithLogo(makeTestLogo(8, 8, red), 0.2))
	assertNoError(t, err)
	svg := string(data)
	assertEqual(t, 1, code.Version())
	assertContains(t, svg, `<rect x="120" y="120" width="50" height="50" fill="#FFFFFF"/>`)
	assertContains(t, svg, `<image x="130" y="130" width="30" height="30" href="data:image/png;base64,`)
	if strings.Index(svg, "<image") > strings.Index(svg, "</svg>") {
		t.Error("logo is outside the <svg> element")
	}
}

func TestLogoSVGTransparentBackground(t *testing.T) {
	code, _ := Encode("Hello, world!", WithECC(ECCHigh))
	data, err := code.SVG(WithBackground(color.Transparent), WithLogo(makeTestLogo(8, 8, red), 0.2))
	assertNoError(t, err)
	if strings.Contains(string(data), "<rect") {
		t.Error("transparent background still emits a pad <rect>")
	}
}

func TestLogoTooLarge(t *testing.T) {
	logo := makeTestLogo(10, 10, red)
	low, _ := Encode("Hello, world!", WithECC(ECCLow), WithoutECCBoost())
	high, _ := Encode("Hello, world!", WithECC(ECCHigh))

	for _, tc := range []struct {
		name  string
		code  *Code
		ratio float64
		ok    bool
	}{
		{"low ECC, small logo", low, 0.2, false},
		{"high ECC, small logo", high, 0.2, true},
		{"high ECC, huge logo", high, 0.7, false},
		{"logo covers whole symbol", high, 0.99, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := []RenderOption{WithLogo(logo, tc.ratio)}
			_, errImg := tc.code.Image(opts...)
			_, errPNG := tc.code.PNG(opts...)
			_, errSVG := tc.code.SVG(opts...)
			for _, err := range []error{errImg, errPNG, errSVG} {
				if tc.ok && err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !tc.ok && !errors.Is(err, ErrLogoTooLarge) {
					t.Errorf("error = %v, want ErrLogoTooLarge", err)
				}
			}
		})
	}
}

func TestEccRecoveryBudget(t *testing.T) {
	assertInDelta(t, 0.05, eccRecoveryBudget(ECCLow), 1e-9)
	assertInDelta(t, 0.12, eccRecoveryBudget(ECCMedium), 1e-9)
	assertInDelta(t, 0.20, eccRecoveryBudget(ECCQuartile), 1e-9)
	assertInDelta(t, 0.25, eccRecoveryBudget(ECCHigh), 1e-9)
}

func TestLogoBoxParity(t *testing.T) {
	for ver := MinVersion; ver <= MaxVersion; ver++ {
		size := 4*ver + 17
		for _, ratio := range []float64{0.01, 0.15, 0.2, 0.33} {
			box := (&logoConfig{ratio: ratio}).boxModules(size)
			if box%2 != size%2 || box < 3 {
				t.Fatalf("version %d ratio %v: box %d modules does not center on a %d-module symbol", ver, ratio, box, size)
			}
		}
	}
}
