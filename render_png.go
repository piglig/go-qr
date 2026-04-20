package go_qr

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
)

// PNG renders the QR code to the given file path.
func (q *QrCode) PNG(config *QrCodeImgConfig, filePath string) error {
	if err := q.validateWritePNGConfig(config); err != nil {
		return err
	}

	pngFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating PNG file: %w", err)
	}
	defer pngFile.Close()

	return q.encodePNG(config, pngFile)
}

// WriteAsPNG renders the QR code as PNG to the provided io.Writer.
func (q *QrCode) WriteAsPNG(config *QrCodeImgConfig, writer io.Writer) error {
	if err := q.validateWritePNGConfig(config); err != nil {
		return err
	}
	return q.encodePNG(config, writer)
}

// ToPNGBytes renders the QR code as PNG and returns the bytes in memory.
// Useful for HTTP handlers, serverless functions, or any case where writing
// to a file is unnecessary.
func (q *QrCode) ToPNGBytes(config *QrCodeImgConfig) ([]byte, error) {
	if err := q.validateWritePNGConfig(config); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := q.encodePNG(config, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ToImage renders the QR code into an *image.RGBA using the provided config.
// This is the composition primitive used by PNG output; callers can use it
// directly to overlay logos or perform other image-space transformations
// before encoding to their preferred format.
func (q *QrCode) ToImage(config *QrCodeImgConfig) (*image.RGBA, error) {
	if err := q.validateWritePNGConfig(config); err != nil {
		return nil, err
	}
	return q.renderImage(config)
}

// renderImage is the shared composition primitive: it paints modules into an
// *image.RGBA and, if a logo is configured, validates and overlays it. The
// caller has already validated the config.
func (q *QrCode) renderImage(config *QrCodeImgConfig) (*image.RGBA, error) {
	rgba := q.paintModules(config)
	if logo := config.options.logo; logo != nil {
		if err := logo.validate(q, config.scale, config.border); err != nil {
			return nil, err
		}
		if err := logo.overlayOnImage(rgba, q.GetSize(), config.scale, config.border); err != nil {
			return nil, err
		}
	}
	return rgba, nil
}

// encodePNG renders to an image and writes PNG bytes to writer.
func (q *QrCode) encodePNG(config *QrCodeImgConfig, writer io.Writer) error {
	rgba, err := q.renderImage(config)
	if err != nil {
		return err
	}
	if err := png.Encode(writer, rgba); err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}
	return nil
}

// validateWritePNGConfig validates the parameters to write the QR code as an image.
func (q *QrCode) validateWritePNGConfig(config *QrCodeImgConfig) error {
	if err := config.Valid(); err != nil {
		return err
	}
	// Ensure that the border size combined with QR code size does not exceed
	// the maximum allowed integer value after scaling.
	if config.border > (math.MaxInt32/2) || int64(q.GetSize())+int64(config.border)*2 > math.MaxInt32/int64(config.scale) {
		return fmt.Errorf("%w: scale or border too large", ErrInvalidConfig)
	}
	return nil
}

// paintModules allocates an RGBA image and fills each pixel with the dark or
// light color according to the QR module at that position.
func (q *QrCode) paintModules(config *QrCodeImgConfig) *image.RGBA {
	size := q.GetSize() + config.border*2
	imageWidth := size * config.scale
	imageHeight := size * config.scale
	result := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			moduleX := x/config.scale - config.border
			moduleY := y/config.scale - config.border
			if q.GetModule(moduleX, moduleY) {
				result.Set(x, y, config.Dark())
			} else {
				result.Set(x, y, config.Light())
			}
		}
	}
	return result
}
