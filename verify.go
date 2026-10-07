package qr

import (
	"bytes"
	"fmt"
	"image/color"
)

// minSymbolContrast is the smallest luminance difference between the light
// and dark modules, as a fraction of full scale, that Verify accepts. It is
// the ISO/IEC 15415 symbol contrast threshold for grade C; phone cameras
// struggle below it even though this package's decoder may not.
const minSymbolContrast = 0.40

// Verify renders the code with opts, as Image would, decodes the result and
// checks that it carries exactly the code's data. It also rejects color
// choices that phone scanners commonly fail on even when this package's
// decoder succeeds: a foreground that is lighter than the background, or
// less than 40% luminance contrast between them. Use it to check custom
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
	fg, bg := luminanceOverWhite(cfg.fg), luminanceOverWhite(cfg.bg)
	switch contrast := bg - fg; {
	case contrast < 0:
		return fmt.Errorf("%w: foreground is lighter than background; many scanners cannot read inverted codes", ErrUnreadable)
	case contrast < minSymbolContrast:
		return fmt.Errorf("%w: contrast %.0f%% is below the %.0f%% scanners need", ErrUnreadable, contrast*100, minSymbolContrast*100)
	}

	img, err := c.renderRGBA(&cfg)
	if err != nil {
		return err
	}
	want, _, _, _, err := decodeMatrix(c.modules)
	if err != nil {
		return fmt.Errorf("%w: the symbol's own modules do not decode: %v", ErrUnreadable, err)
	}
	got, err := searchImage(img, decodeConfig{}, func(grid [][]bool) (*DecodeResult, error) {
		data, ver, ecc, mask, err := decodeMatrix(grid)
		if err != nil {
			return nil, err
		}
		return &DecodeResult{Version: ver, ECC: ecc, Mask: mask, codewords: data}, nil
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if !bytes.Equal(got.codewords, want) {
		return fmt.Errorf("%w: rendering decodes to different data", ErrUnreadable)
	}
	return nil
}

// luminanceOverWhite returns the relative luminance, from 0 to 1, of c
// composited over white.
func luminanceOverWhite(c color.Color) float64 {
	r, g, b, a := c.RGBA()
	return float64(lumaPremul(r>>8, g>>8, b>>8, a>>8)) / 255
}
