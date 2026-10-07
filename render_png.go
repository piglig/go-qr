package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
)

// Image renders the symbol, quiet zone and logo into a new RGBA image. Use it
// to compose the code with other graphics before encoding.
func (c *Code) Image(opts ...RenderOption) (*image.RGBA, error) {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return nil, err
	}
	return c.renderRGBA(&cfg)
}

func (c *Code) renderRGBA(cfg *renderConfig) (*image.RGBA, error) {
	side, err := cfg.sidePixels(c)
	if err != nil {
		return nil, err
	}
	if err := cfg.checkLogo(c); err != nil {
		return nil, err
	}
	var img *image.RGBA
	if cfg.styled() {
		img = paintStyled(c, cfg, side)
	} else {
		img = paintRGBA(c, cfg, side)
	}
	if cfg.logo != nil {
		cfg.logo.overlay(img, c.Size(), cfg)
	}
	return img, nil
}

// WritePNG writes the symbol as a PNG image to w.
func (c *Code) WritePNG(w io.Writer, opts ...RenderOption) error {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return err
	}

	var img image.Image
	if cfg.logo != nil {
		if img, err = c.renderRGBA(&cfg); err != nil {
			return err
		}
	} else {
		side, err := cfg.sidePixels(c)
		if err != nil {
			return err
		}
		if cfg.styled() {
			img = paintStyledPaletted(c, &cfg, side)
		} else {
			img = paintPaletted(c, &cfg, side)
		}
	}

	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("qr: write PNG: %w", err)
	}
	return nil
}

// PNG returns the symbol encoded as a PNG image.
func (c *Code) PNG(opts ...RenderOption) ([]byte, error) {
	var buf bytes.Buffer
	if err := c.WritePNG(&buf, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
