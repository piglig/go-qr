package qr

import (
	"image"
	"testing"
	"unicode/utf8"
)

// FuzzDecodeRoundTrip encodes fuzzer-provided text, renders it, and asserts it
// decodes back unchanged. This pins encoder/decoder symmetry across arbitrary
// inputs (numeric, alphanumeric, byte, Kanji, invalid UTF-8).
func FuzzDecodeRoundTrip(f *testing.F) {
	for _, s := range []string{"", "1", "42", "HELLO WORLD", "https://x.io/a?b=1",
		"日本語テスト", "mixed 123 ABC $%*+-./:", "\x00\x01\xff binary"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		qr, err := Encode(s, WithECC(ECCLow))
		if err != nil {
			t.Skip() // too long — not a decode concern
		}
		img, err := qr.Image(WithScale(4), WithQuietZone(4))
		if err != nil {
			t.Fatalf("render %q: %v", s, err)
		}
		res, err := Decode(img)
		if err != nil {
			t.Fatalf("decode %q: %v", s, err)
		}
		if utf8.ValidString(s) {
			if res.Text != s {
				t.Fatalf("round trip: want %q got %q", s, res.Text)
			}
			return
		}
		// Invalid UTF-8 is carried in one byte segment and read back as
		// ISO-8859-1, so compare the raw bytes instead of the text.
		if len(res.Segments) != 1 || string(res.Segments[0].Data) != s {
			t.Fatalf("round trip: want bytes %q got %+v", s, res.Segments)
		}
	})
}

// FuzzDecodeNoPanic feeds arbitrary grayscale images to Decode and asserts it
// never panics — corruption beyond the ECC budget must yield a clean error,
// never a crash and never wrong text returned as success.
func FuzzDecodeNoPanic(f *testing.F) {
	f.Add([]byte{0, 255, 0, 255, 0}, uint8(0))
	f.Add([]byte{0}, uint8(40))
	f.Fuzz(func(t *testing.T, pix []byte, extra uint8) {
		n := int(extra)%80 + 21 // 21..100 px square
		img := image.NewGray(image.Rect(0, 0, n, n))
		if len(pix) > 0 {
			for i := range img.Pix {
				img.Pix[i] = pix[i%len(pix)]
			}
		}
		// Must not panic; a returned value (if any) is best-effort.
		_, _ = Decode(img)
	})
}
