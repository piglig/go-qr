// Package verify decodes a QR code image and exposes a round-trip helper for
// asserting that generated QR codes are actually scannable.
//
// As of go-qr's native decoder it wraps go_qr.Decode directly, so this package
// (and the generator's --verify mode) no longer pulls in any third-party
// decoder. The gozxing dependency now lives only in tools/bench, where it
// serves as a cross-check oracle for the benchmark suite.
package verify

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	go_qr "github.com/piglig/go-qr"
)

// Decode returns the text content of a QR code rendered in the given image.
func Decode(img image.Image) (string, error) {
	text, err := go_qr.Decode(img)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return text, nil
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
