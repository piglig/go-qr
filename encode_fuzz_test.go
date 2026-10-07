package qr

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"
	"unicode/utf8"
)

// Flag bits for FuzzEncode.
const (
	fuzzSimple = 1 << iota
	fuzzUTF8ECI
	fuzzGS1
	fuzzNoBoost
	fuzzFixedMask // mask taken from the top three bits
)

// FuzzEncode encodes arbitrary text under arbitrary options and checks that
// encoding either succeeds or reports ErrDataTooLong, that the symbol
// honors the options, and that it decodes back to the same data.
func FuzzEncode(f *testing.F) {
	for _, s := range []string{"", "0", "HELLO 42", "https://x.io/?a=%1", "漢字テキスト", "é\x1dGS", "\xff\xfe", strings.Repeat("9", 300)} {
		f.Add(s, uint8(0), uint8(0))
		f.Add(s, uint8(3), uint8(fuzzUTF8ECI|fuzzGS1))
		f.Add(s, uint8(1), uint8(fuzzSimple|fuzzNoBoost|fuzzFixedMask|5<<5))
	}
	f.Fuzz(func(t *testing.T, text string, eccIdx, flags uint8) {
		ecc := ECC(eccIdx % 4)
		opts := []EncodeOption{WithECC(ecc)}
		if flags&fuzzSimple != 0 {
			opts = append(opts, WithSimpleSegmentation())
		}
		if flags&fuzzUTF8ECI != 0 {
			opts = append(opts, WithUTF8ECI())
		}
		gs1 := flags&fuzzGS1 != 0
		if gs1 {
			opts = append(opts, WithGS1())
		}
		if flags&fuzzNoBoost != 0 {
			opts = append(opts, WithoutECCBoost())
		}
		mask := -1
		if flags&fuzzFixedMask != 0 {
			mask = int(flags >> 5)
			opts = append(opts, WithMask(mask))
		}

		code, err := Encode(text, opts...)
		if err != nil {
			if !errors.Is(err, ErrDataTooLong) {
				t.Fatalf("Encode(%q): unexpected error %v", text, err)
			}
			return
		}
		switch {
		case code.Size() != 4*code.Version()+17:
			t.Fatalf("size %d does not match version %d", code.Size(), code.Version())
		case code.ECC() < ecc || flags&fuzzNoBoost != 0 && code.ECC() != ecc:
			t.Fatalf("ECC %v, requested %v (boost %v)", code.ECC(), ecc, flags&fuzzNoBoost == 0)
		case mask >= 0 && code.Mask() != mask:
			t.Fatalf("mask %d, requested %d", code.Mask(), mask)
		}

		img, err := code.Image(WithScale(1))
		if err != nil {
			t.Fatal(err)
		}
		res, err := Decode(img, WithFastPathOnly())
		if err != nil {
			t.Fatalf("Decode(%q): %v", text, err)
		}
		if res.GS1 != gs1 || res.Version != code.Version() || res.ECC != code.ECC() || res.Mask != code.Mask() {
			t.Fatalf("metadata %+v does not match the code", res)
		}
		if utf8.ValidString(text) {
			if res.Text != text {
				t.Fatalf("round trip: want %q, got %q", text, res.Text)
			}
			return
		}
		// Invalid UTF-8 travels in byte segments and may be read back as
		// ISO-8859-1, so compare the raw payload.
		var raw []byte
		for _, s := range res.Segments {
			if s.Mode != ModeECI && s.Mode != ModeFNC1 {
				raw = append(raw, s.Data...)
			}
		}
		if string(raw) != text {
			t.Fatalf("round trip: want bytes %q, got %q", text, raw)
		}
	})
}

// FuzzEncodeStructured splits arbitrary text over small symbols and checks
// that the sequence reassembles.
func FuzzEncodeStructured(f *testing.F) {
	f.Add("short", uint8(0))
	f.Add(strings.Repeat("structured append 漢字 ", 20), uint8(2))
	f.Add(strings.Repeat("x", 300), uint8(0))
	f.Fuzz(func(t *testing.T, text string, maxVer uint8) {
		if !utf8.ValidString(text) {
			t.Skip() // the split is by character
		}
		codes, err := EncodeStructured(text, WithECC(ECCLow), WithVersionRange(1, 1+int(maxVer%6)))
		if err != nil {
			if !errors.Is(err, ErrDataTooLong) {
				t.Fatalf("EncodeStructured: unexpected error %v", err)
			}
			return
		}
		results := make([]*DecodeResult, len(codes))
		for i, c := range codes {
			img, err := c.Image(WithScale(1))
			if err != nil {
				t.Fatal(err)
			}
			if results[i], err = Decode(img, WithFastPathOnly()); err != nil {
				t.Fatalf("symbol %d: %v", i, err)
			}
		}
		if len(codes) == 1 {
			if results[0].Text != text {
				t.Fatalf("single symbol: want %q, got %q", text, results[0].Text)
			}
			return
		}
		got, err := JoinStructuredAppend(results...)
		if err != nil {
			t.Fatal(err)
		}
		if got != text {
			t.Fatalf("joined: want %q, got %q", text, got)
		}
	})
}

// FuzzRender renders under arbitrary scales, quiet zones and styles and
// checks that every output is well formed and sized as configured.
func FuzzRender(f *testing.F) {
	f.Add("render", uint8(0), uint8(4), uint8(0))
	f.Add("styled 123", uint8(2), uint8(0), uint8(0x15))
	f.Fuzz(func(t *testing.T, text string, scale, quietZone, style uint8) {
		code, err := Encode(text, WithECC(ECCLow))
		if err != nil {
			return
		}
		opts := []RenderOption{
			WithScale(1 + int(scale%6)),
			WithQuietZone(int(quietZone % 6)),
			WithModuleShape(ModuleShape(style % 3)),
			WithFinderShape(FinderShape(style / 3 % 3)),
		}
		if style&0x40 != 0 {
			opts = append(opts, WithGradient(navy, teal, float64(style)))
		}
		if style&0x80 != 0 {
			opts = append(opts, WithFinderColor(teal, navy))
		}
		side := (code.Size() + 2*int(quietZone%6)) * (1 + int(scale%6))

		img, err := code.Image(opts...)
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != side || b.Dy() != side {
			t.Fatalf("image is %v, want %dx%d", b, side, side)
		}
		data, err := code.PNG(opts...)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != side || cfg.Height != side {
			t.Fatalf("PNG config %+v, err %v; want %dx%d", cfg, err, side, side)
		}
		svg, err := code.SVG(opts...)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(svg, []byte("<svg")) || !bytes.HasSuffix(svg, []byte("</svg>\n")) {
			t.Fatalf("malformed SVG: %.80q", svg)
		}
	})
}
