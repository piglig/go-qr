package qr

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func makeTestLogo(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestWithLogo_PNG(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.2))
	b, err := qr.ToPNGBytes(cfg)
	assertNoError(t, err)
	assertNotEmpty(t, b)

	// Decode and verify the center pixel is red (logo was drawn).
	decoded, err := png.Decode(bytes.NewReader(b))
	assertNoError(t, err)
	cx := decoded.Bounds().Dx() / 2
	cy := decoded.Bounds().Dy() / 2
	r, g, bl, _ := decoded.At(cx, cy).RGBA()
	assertEqual(t, uint32(0xffff), r)
	assertEqual(t, uint32(0), g)
	assertEqual(t, uint32(0), bl)
}

func TestWithLogo_SVG(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.RGBA{R: 0, G: 128, B: 255, A: 255})

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.2))
	b, err := qr.ToSVGBytes(cfg)
	assertNoError(t, err)
	s := string(b)
	assertContains(t, s, "<image")
	assertContains(t, s, "data:image/png;base64,")
	// The logo fragment must be inside the svg element.
	imgIdx := strings.Index(s, "<image")
	endIdx := strings.LastIndex(s, "</svg>")
	assertTrue(t, imgIdx > 0 && imgIdx < endIdx, "logo must be placed before </svg>")
}

func TestWithLogo_SVG_Optimal(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	cfg := NewQrCodeImgConfig(10, 4, WithOptimalSVG(), WithLogo(logo, 0.18))
	b, err := qr.ToSVGBytes(cfg)
	assertNoError(t, err)
	s := string(b)
	assertContains(t, s, "fill-rule=\"evenodd\"")
	assertContains(t, s, "<image")
}

func TestWithLogo_ExceedsECCBudget(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCLow)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	// sizeRatio 0.7 is large enough to exceed every ECC budget, including ECCHigh.
	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.7))
	_, err = qr.ToPNGBytes(cfg)
	assertError(t, err)
	assertContains(t, err.Error(), "exceeds ECC")
}

func TestWithLogo_InvalidSizeRatio(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	cases := []float64{0, -0.1, 1.0, 1.5}
	for _, r := range cases {
		cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, r))
		_, err := qr.ToPNGBytes(cfg)
		assertError(t, err, "sizeRatio %v should fail", r)
	}
}

func TestWithLogo_NilImage(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(nil, 0.2))
	_, err = qr.ToPNGBytes(cfg)
	assertError(t, err)
}

func TestWithLogo_HigherECCAllowsLargerLogo(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	// Ratio 0.22 = ~5.3% occlusion (1-module padding adds slightly more). OK for ECCHigh.
	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.22))
	_, err = qr.ToPNGBytes(cfg)
	assertNoError(t, err)
}

func TestWithLogo_ImageAPIIncludesLogo(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCHigh)
	assertNoError(t, err)
	logo := makeTestLogo(40, 40, color.RGBA{R: 10, G: 200, B: 20, A: 255})

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.2))
	img, err := qr.ToImage(cfg)
	assertNoError(t, err)

	cx := img.Bounds().Dx() / 2
	cy := img.Bounds().Dy() / 2
	r, g, b, _ := img.At(cx, cy).RGBA()
	assertEqual(t, uint32(0x0a0a), r)
	assertEqual(t, uint32(0xc8c8), g)
	assertEqual(t, uint32(0x1414), b)
}
