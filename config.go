package go_qr

import (
	"fmt"
	"image/color"
)

// Option configures a QrCodeImgConfig. Pass options to NewQrCodeImgConfig.
type Option func(*QrCodeImgConfig)

// QrCodeImgConfig is the representation of the QR Code generation configuration.
type QrCodeImgConfig struct {
	scale, border int
	light, dark   color.Color
	svgXMLHeader  bool
	optimalSVG    bool
	logo          *logoConfig
}

// NewQrCodeImgConfig creates a QR code generation config with the provided scale
// and border. The default light color is white and the default dark color is black;
// use WithLight / WithDark to override.
func NewQrCodeImgConfig(scale int, border int, options ...Option) *QrCodeImgConfig {
	config := &QrCodeImgConfig{scale: scale, border: border, light: color.White, dark: color.Black}
	for _, o := range options {
		o(config)
	}
	return config
}

// valid reports whether the config is suitable for rendering. It is checked
// internally by every render entry point, so callers never invoke it directly.
func (q *QrCodeImgConfig) valid() error {
	if q.scale <= 0 {
		return fmt.Errorf("%w: scale must be positive", ErrInvalidConfig)
	}

	if q.border < 0 {
		return fmt.Errorf("%w: border must be non-negative", ErrInvalidConfig)
	}
	return nil
}

// Light returns the light (background) color.
func (q *QrCodeImgConfig) Light() color.Color {
	return q.light
}

// Dark returns the dark (foreground) color.
func (q *QrCodeImgConfig) Dark() color.Color {
	return q.dark
}
