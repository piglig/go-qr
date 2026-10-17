package qr

import (
	"bytes"
	"fmt"
	"image/color"

	"github.com/piglig/go-qr/v2/internal/detect"
	"github.com/piglig/go-qr/v2/internal/render"
	"github.com/piglig/go-qr/v2/internal/symbol"
)

// minSymbolContrast is the smallest luminance difference between the light
// and dark modules, as a fraction of full scale, that Verify accepts. It is
// the ISO/IEC 15415 symbol contrast threshold for grade C; phone cameras
// struggle below it even though this package's decoder may not.
const minSymbolContrast = 0.40

// Verify renders the code with opts, as Image would, decodes the result and
// checks that it carries exactly the code's data. It also rejects color
// choices that phone scanners commonly fail on even when this package's
// decoder succeeds: any dark color (the foreground, both ends of a
// gradient, finder colors) that is lighter than the background or has less
// than 40% luminance contrast with it. Use it to check custom
// colors or a logo before publishing a code.
//
// Verify checks the raster rendering; SVG output with the same options draws
// the same modules. The error wraps ErrUnreadable when the rendering is not
// readable, or ErrInvalidArgument or ErrLogoTooLarge for invalid options.
func (c *Code) Verify(opts ...RenderOption) error {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return err
	}
	bg := luminanceOverWhite(cfg.BG)
	for _, dark := range cfg.DarkColors() {
		switch contrast := bg - luminanceOverWhite(dark); {
		case contrast < 0:
			return fmt.Errorf("%w: dark color %s is lighter than the background; many scanners cannot read inverted codes", ErrUnreadable, render.ColorToSVG(dark))
		case contrast < minSymbolContrast:
			return fmt.Errorf("%w: contrast of %s is %.0f%%, below the %.0f%% scanners need", ErrUnreadable, render.ColorToSVG(dark), contrast*100, minSymbolContrast*100)
		}
	}

	img, err := c.renderRGBA(&cfg)
	if err != nil {
		return err
	}
	want, err := symbol.ReadMatrix(c.modules, nil)
	if err != nil {
		return fmt.Errorf("%w: the symbol's own modules do not decode: %v", ErrUnreadable, err)
	}
	// Only hard decisions count: a design must scan with readers that do
	// not decode softly, as most do not.
	got, err := searchImage(img, decodeConfig{}, func(grid [][]bool, rel []float32) (*DecodeResult, error) {
		if rel != nil {
			return nil, fmt.Errorf("%w: only soft decoding reads it", ErrDecodeFailed)
		}
		sd, err := symbol.ReadMatrix(grid, nil)
		if err != nil {
			return nil, err
		}
		return &DecodeResult{Version: sd.Version, ECC: ECC(sd.ECC), Mask: sd.Mask, codewords: sd.Codewords}, nil
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if !bytes.Equal(got.codewords, want.Codewords) {
		return fmt.Errorf("%w: rendering decodes to different data", ErrUnreadable)
	}
	return nil
}

// luminanceOverWhite returns the relative luminance, from 0 to 1, of c
// composited over white.
func luminanceOverWhite(c color.Color) float64 {
	r, g, b, a := c.RGBA()
	return float64(detect.LumaPremul(r>>8, g>>8, b>>8, a>>8)) / 255
}
