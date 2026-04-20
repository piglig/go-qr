package go_qr

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.2))
	b, err := qr.ToPNGBytes(cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, b)

	// Decode and verify the center pixel is red (logo was drawn).
	decoded, err := png.Decode(bytes.NewReader(b))
	assert.NoError(t, err)
	cx := decoded.Bounds().Dx() / 2
	cy := decoded.Bounds().Dy() / 2
	r, g, bl, _ := decoded.At(cx, cy).RGBA()
	assert.Equal(t, uint32(0xffff), r)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), bl)
}

func TestWithLogo_SVG(t *testing.T) {
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.RGBA{R: 0, G: 128, B: 255, A: 255})

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.2))
	b, err := qr.ToSVGBytes(cfg)
	assert.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "<image")
	assert.Contains(t, s, "data:image/png;base64,")
	// The logo fragment must be inside the svg element.
	imgIdx := strings.Index(s, "<image")
	endIdx := strings.LastIndex(s, "</svg>")
	assert.True(t, imgIdx > 0 && imgIdx < endIdx, "logo must be placed before </svg>")
}

func TestWithLogo_SVG_Optimal(t *testing.T) {
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	cfg := NewQrCodeImgConfig(10, 4, WithOptimalSVG(), WithLogo(logo, 0.18))
	b, err := qr.ToSVGBytes(cfg)
	assert.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "fill-rule=\"evenodd\"")
	assert.Contains(t, s, "<image")
}

func TestWithLogo_ExceedsECCBudget(t *testing.T) {
	qr, err := EncodeText("Hello, world!", Low)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	// sizeRatio 0.7 is large enough to exceed every ECC budget, including High.
	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.7))
	_, err = qr.ToPNGBytes(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds ECC")
}

func TestWithLogo_InvalidSizeRatio(t *testing.T) {
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	cases := []float64{0, -0.1, 1.0, 1.5}
	for _, r := range cases {
		cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, r))
		_, err := qr.ToPNGBytes(cfg)
		assert.Error(t, err, "sizeRatio %v should fail", r)
	}
}

func TestWithLogo_NilImage(t *testing.T) {
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(nil, 0.2))
	_, err = qr.ToPNGBytes(cfg)
	assert.Error(t, err)
}

func TestWithLogo_HigherECCAllowsLargerLogo(t *testing.T) {
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.Black)

	// Ratio 0.22 = ~5.3% occlusion (1-module padding adds slightly more). OK for High.
	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.22))
	_, err = qr.ToPNGBytes(cfg)
	assert.NoError(t, err)
}

func TestWithLogo_ImageAPIIncludesLogo(t *testing.T) {
	qr, err := EncodeText("Hello, world!", High)
	assert.NoError(t, err)
	logo := makeTestLogo(40, 40, color.RGBA{R: 10, G: 200, B: 20, A: 255})

	cfg := NewQrCodeImgConfig(10, 4, WithLogo(logo, 0.2))
	img, err := qr.ToImage(cfg)
	assert.NoError(t, err)

	cx := img.Bounds().Dx() / 2
	cy := img.Bounds().Dy() / 2
	r, g, b, _ := img.At(cx, cy).RGBA()
	assert.Equal(t, uint32(0x0a0a), r)
	assert.Equal(t, uint32(0xc8c8), g)
	assert.Equal(t, uint32(0x1414), b)
}
