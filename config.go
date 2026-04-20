package go_qr

import (
	"fmt"
	"image/color"
)

// QrCodeImgConfig is the representation of the QR Code generation configuration.
type QrCodeImgConfig struct {
	scale, border int
	light, dark   color.Color
	options       *qrCodeConfig
}

// NewQrCodeImgConfig creates a QR code generation config with the provided scale
// and border. The default light color is white and the default dark color is black;
// use WithLight / WithDark to override.
func NewQrCodeImgConfig(scale int, border int, options ...func(config *QrCodeImgConfig)) *QrCodeImgConfig {
	config := &QrCodeImgConfig{scale: scale, border: border, light: color.White, dark: color.Black, options: &qrCodeConfig{}}
	for _, o := range options {
		o(config)
	}
	return config
}

// Valid reports whether the config is suitable for rendering.
func (q *QrCodeImgConfig) Valid() error {
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

// SetLight sets the light (background) color.
func (q *QrCodeImgConfig) SetLight(light color.Color) {
	q.light = light
}

// Dark returns the dark (foreground) color.
func (q *QrCodeImgConfig) Dark() color.Color {
	return q.dark
}

// SetDark sets the dark (foreground) color.
func (q *QrCodeImgConfig) SetDark(dark color.Color) {
	q.dark = dark
}
