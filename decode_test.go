package go_qr

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

// TestDecodeRoundTrip encodes text, renders to an image, then decodes it back.
func TestDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		text string
		ecl  Ecc
	}{
		{"numeric", "12345678901234567890", Low},
		{"alnum", "HELLO WORLD 42 $%*+-./:", Medium},
		{"byte_url", "https://github.com/piglig/go-qr?x=1&y=2", Quartile},
		{"byte_utf8", "héllo wörld — 日本語テスト", High},
		{"wifi", "WIFI:T:WPA;S:home-network;P:s3cret-pass;;", Medium},
		{"long", strings.Repeat("The quick brown fox 0123456789. ", 8), High},
	}

	scales := []int{1, 4, 10}
	for _, tc := range cases {
		for _, scale := range scales {
			t.Run(tc.name+"/scale"+itoa(scale), func(t *testing.T) {
				qr, err := EncodeText(tc.text, tc.ecl)
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				img, err := qr.ToImage(NewQrCodeImgConfig(scale, 4))
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				got, err := Decode(img)
				if err != nil {
					t.Fatalf("decode: %v", err)
				}
				if got != tc.text {
					t.Fatalf("round-trip mismatch:\n want %q\n  got %q", tc.text, got)
				}
			})
		}
	}
}

// TestDecodeDetailed checks the structured metadata.
func TestDecodeDetailed(t *testing.T) {
	qr, err := EncodeText("HELLO", Quartile)
	if err != nil {
		t.Fatal(err)
	}
	img, err := qr.ToImage(NewQrCodeImgConfig(6, 4))
	if err != nil {
		t.Fatal(err)
	}
	res, err := DecodeDetailed(img)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "HELLO" {
		t.Errorf("text: got %q", res.Text)
	}
	if res.Version != qr.version {
		t.Errorf("version: got %d want %d", res.Version, qr.version)
	}
	// EncodeText boosts ECC for tiny payloads, so compare against the actual
	// level the encoder settled on, not the requested one.
	if res.Ecc != qr.errorCorrectionLevel {
		t.Errorf("ecc: got %d want %d", res.Ecc, qr.errorCorrectionLevel)
	}
	if res.Mask != qr.mask {
		t.Errorf("mask: got %d want %d", res.Mask, qr.mask)
	}
}

// TestRSCorrectsErrors verifies the Reed-Solomon decoder repairs corrupted
// modules up to the ECC budget (this is the path clean images never exercise).
func TestRSCorrectsErrors(t *testing.T) {
	qr, err := EncodeText("ERROR CORRECTION TEST 123", High) // High = ~30% recovery
	if err != nil {
		t.Fatal(err)
	}
	img, err := qr.ToImage(NewQrCodeImgConfig(8, 4))
	if err != nil {
		t.Fatal(err)
	}
	modules, err := fastSample(img)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a handful of data modules near the center (avoid finders).
	c := len(modules) / 2
	flips := 0
	for dy := -2; dy <= 2 && flips < 6; dy++ {
		for dx := -2; dx <= 2 && flips < 6; dx++ {
			modules[c+dy][c+dx] = !modules[c+dy][c+dx]
			flips++
		}
	}
	data, ver, _, _, err := decodeMatrix(modules)
	if err != nil {
		t.Fatalf("decodeMatrix after corruption: %v", err)
	}
	text, _, err := parseBitstream(data, ver)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if text != "ERROR CORRECTION TEST 123" {
		t.Fatalf("RS failed to recover: got %q", text)
	}
}

// TestDecodeRotated exercises the robust path: render, rotate by a few degrees
// about the center (white padding), and decode via finder detection + affine.
func TestDecodeRotated(t *testing.T) {
	cases := []struct {
		text  string
		ecl   Ecc
		theta float64 // radians
	}{
		{"ROTATED QR 12345", Medium, 5 * math.Pi / 180},
		{"https://example.com/x", Quartile, -8 * math.Pi / 180},
		{"hello rotated world", High, 12 * math.Pi / 180},
	}
	for _, tc := range cases {
		qr, err := EncodeText(tc.text, tc.ecl)
		if err != nil {
			t.Fatal(err)
		}
		img, err := qr.ToImage(NewQrCodeImgConfig(8, 6)) // generous quiet zone
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(rotateGray(img, tc.theta))
		if err != nil {
			t.Errorf("%q @ %.0f°: decode failed: %v", tc.text, tc.theta*180/math.Pi, err)
			continue
		}
		if got != tc.text {
			t.Errorf("%q @ %.0f°: got %q", tc.text, tc.theta*180/math.Pi, got)
		}
	}
}

// rotateGray rotates about the center with white fill (nearest-neighbour).
func rotateGray(src *image.RGBA, theta float64) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	sin, cos := math.Sin(theta), math.Cos(theta)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			sx := int(cos*dx + sin*dy + cx + 0.5)
			sy := int(-sin*dx + cos*dy + cy + 0.5)
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				dst.SetGray(x, y, color.Gray{Y: 255})
				continue
			}
			r, g, bl, _ := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
			luma := (299*r + 587*g + 114*bl) / 1000 / 257
			dst.SetGray(x, y, color.Gray{Y: uint8(luma)})
		}
	}
	return dst
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
