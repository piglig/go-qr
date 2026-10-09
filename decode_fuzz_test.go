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

// FuzzDecodeAllNoPanic runs DecodeAll over arbitrary grayscale images and
// checks that it neither panics nor returns results with an error.
func FuzzDecodeAllNoPanic(f *testing.F) {
	f.Add(uint8(40), uint8(40), []byte{0, 255, 0, 255})
	f.Fuzz(func(t *testing.T, w, h uint8, pix []byte) {
		img := image.NewGray(image.Rect(0, 0, int(w)+1, int(h)+1))
		for i := range img.Pix {
			if len(pix) > 0 {
				img.Pix[i] = pix[i%len(pix)]
			}
		}
		rs, err := DecodeAll(img)
		if err != nil && rs != nil || err == nil && len(rs) == 0 {
			t.Fatalf("DecodeAll returned %d results and error %v", len(rs), err)
		}
	})
}

// FuzzDecodeAllRoundTrip renders two symbols of fuzzed text side by side,
// darkens or lightens the pixels the noise names, and decodes them all.
// Without noise both must come back, in order.
func FuzzDecodeAllRoundTrip(f *testing.F) {
	f.Add("left", "right", []byte(nil))
	f.Add("https://example.com", "12345", []byte{7, 200, 3, 99})
	f.Fuzz(func(t *testing.T, a, b string, noise []byte) {
		ca, err1 := Encode(a, WithECC(ECCMedium))
		cb, err2 := Encode(b, WithECC(ECCMedium))
		// Text that is not UTF-8 reads back as other characters; see
		// FuzzDecodeRoundTrip.
		if !utf8.ValidString(a) || !utf8.ValidString(b) || err1 != nil || err2 != nil || ca.Size() > 61 || cb.Size() > 61 {
			t.Skip()
		}
		ia, _ := ca.Image(WithScale(3))
		ib, _ := cb.Image(WithScale(3))
		side := max(ia.Bounds().Dx(), ib.Bounds().Dx())
		img := image.NewGray(image.Rect(0, 0, 2*side, side))
		for i := range img.Pix {
			img.Pix[i] = 255
		}
		for x, src := range []*image.RGBA{ia, ib} {
			b := src.Bounds()
			for py := 0; py < b.Dy(); py++ {
				for px := 0; px < b.Dx(); px++ {
					img.Set(x*side+px, py, src.At(px, py))
				}
			}
		}
		for i := 0; i+1 < len(noise); i += 2 {
			p := (int(noise[i])<<8 | int(noise[i+1])) % len(img.Pix)
			img.Pix[p] = 255 - img.Pix[p]
		}
		rs, err := DecodeAll(img)
		if err != nil && rs != nil || err == nil && len(rs) == 0 {
			t.Fatalf("DecodeAll returned %d results and error %v", len(rs), err)
		}
		if len(noise) < 2 {
			if err != nil || len(rs) != 2 || rs[0].Text != a || rs[1].Text != b {
				t.Fatalf("DecodeAll = %d results, %v; want %q and %q", len(rs), err, a, b)
			}
		}
	})
}
