package verify

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	go_qr "github.com/piglig/go-qr"
)

func whiteImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	return img
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		text string
		ecl  go_qr.Ecc
	}{
		{"hello", "Hello, world!", go_qr.Low},
		{"url", "https://example.com/path?a=1&b=2", go_qr.Medium},
		{"wifi_payload", "WIFI:T:WPA;S:home;P:s3cret;;", go_qr.Quartile},
		{"long_text", "The quick brown fox jumps over the lazy dog 1234567890", go_qr.High},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qr, err := go_qr.EncodeText(tc.text, tc.ecl)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			b, err := qr.ToPNGBytes(go_qr.NewQrCodeImgConfig(10, 4))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if err := RoundTrip(b, tc.text); err != nil {
				t.Fatalf("round-trip: %v", err)
			}
		})
	}
}

func TestRoundTrip_WithLogo(t *testing.T) {
	text := "https://example.com"
	qr, err := go_qr.EncodeText(text, go_qr.High)
	if err != nil {
		t.Fatal(err)
	}
	logo := whiteImage(40, 40)
	b, err := qr.ToPNGBytes(go_qr.NewQrCodeImgConfig(10, 4, go_qr.WithLogo(logo, 0.2)))
	if err != nil {
		t.Fatal(err)
	}
	if err := RoundTrip(b, text); err != nil {
		t.Fatalf("scan failed with logo: %v", err)
	}
}

func TestDecode_RejectsNonQR(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, whiteImage(100, 100)); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePNG(buf.Bytes()); err == nil {
		t.Fatal("expected error decoding a non-QR image, got nil")
	}
}
