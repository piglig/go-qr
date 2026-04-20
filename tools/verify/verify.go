// Package verify decodes a QR code image and exposes a round-trip helper
// for asserting that generated QR codes are actually scannable.
//
// It lives under the tools submodule so that the main go-qr library keeps
// zero runtime dependencies. Import it from tests, CI jobs, or the
// generator CLI's --verify mode.
package verify

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// Decode returns the text content of a QR code rendered in the given image.
func Decode(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("binary bitmap: %w", err)
	}
	reader := qrcode.NewQRCodeReader()
	result, err := reader.Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return result.GetText(), nil
}

// DecodePNG decodes a QR code from PNG bytes.
func DecodePNG(pngBytes []byte) (string, error) {
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return "", fmt.Errorf("png decode: %w", err)
	}
	return Decode(img)
}

// RoundTrip asserts the given rendered PNG bytes decode back to want.
// Returns an error describing the mismatch if decoding fails or the text
// does not match exactly.
func RoundTrip(pngBytes []byte, want string) error {
	got, err := DecodePNG(pngBytes)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("round-trip mismatch: want %q, got %q", want, got)
	}
	return nil
}
