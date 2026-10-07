package qr

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"
	"testing"
)

// decodeText decodes img and returns the text, failing the test on error.
func decodeText(t *testing.T, img image.Image, opts ...DecodeOption) string {
	t.Helper()
	res, err := Decode(img, opts...)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return res.Text
}

func mustImage(t *testing.T, code *Code, opts ...RenderOption) *image.RGBA {
	t.Helper()
	img, err := code.Image(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func mustEncode(t *testing.T, text string, opts ...EncodeOption) *Code {
	t.Helper()
	code, err := Encode(text, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		text string
		ecc  ECC
	}{
		{"numeric", "12345678901234567890", ECCLow},
		{"alnum", "HELLO WORLD 42 $%*+-./:", ECCMedium},
		{"byte_url", "https://github.com/piglig/go-qr?x=1&y=2", ECCQuartile},
		{"byte_utf8", "héllo wörld — 日本語テスト", ECCHigh},
		{"kanji", "漢字モード、日本語の文章", ECCMedium},
		{"wifi", "WIFI:T:WPA;S:home-network;P:s3cret-pass;;", ECCMedium},
		{"long", strings.Repeat("The quick brown fox 0123456789. ", 8), ECCHigh},
	}
	for _, tc := range cases {
		for _, scale := range []int{1, 4, 10} {
			t.Run(tc.name+"/scale"+strconv.Itoa(scale), func(t *testing.T) {
				img := mustImage(t, mustEncode(t, tc.text, WithECC(tc.ecc)), WithScale(scale))
				if got := decodeText(t, img); got != tc.text {
					t.Fatalf("round trip:\n want %q\n  got %q", tc.text, got)
				}
			})
		}
	}
}

func TestDecodeResultMetadata(t *testing.T) {
	alnum, _ := AlphanumericSegment("HELLO ")
	num, _ := NumericSegment("12345")
	kanji, _ := KanjiSegment("漢字")
	code, err := EncodeSegments([]Segment{alnum, num, BytesSegment([]byte(" ")), kanji}, WithECC(ECCQuartile))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Decode(mustImage(t, code, WithScale(6)))
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "HELLO 12345 漢字", res.Text)
	assertEqual(t, code.Version(), res.Version)
	assertEqual(t, code.ECC(), res.ECC)
	assertEqual(t, code.Mask(), res.Mask)
	assertFalse(t, res.Mirrored)

	var modes []Mode
	for _, s := range res.Segments {
		modes = append(modes, s.Mode)
		assertEqual(t, noECI, s.ECI)
	}
	assertEqual(t, []Mode{ModeAlphanumeric, ModeNumeric, ModeByte, ModeKanji}, modes)
	assertEqual(t, "HELLO ", string(res.Segments[0].Data))
	assertEqual(t, []byte{0x8A, 0xBF, 0x8E, 0x9A}, res.Segments[3].Data) // 漢字 in Shift_JIS
}

// TestRSCorrectsErrors verifies Reed-Solomon repairs corrupted modules.
func TestRSCorrectsErrors(t *testing.T) {
	const text = "ERROR CORRECTION TEST 123"
	img := mustImage(t, mustEncode(t, text, WithECC(ECCHigh)), WithScale(8))
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	l := toLuma(img)
	modules, err := fastSample(l, w, h, otsuThreshold(l), false)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a few data modules near the center, away from the finders.
	c := len(modules) / 2
	for i := 0; i < 6; i++ {
		modules[c-2+i/3][c-2+i%3] = !modules[c-2+i/3][c-2+i%3]
	}
	res, err := decodeGrid(modules)
	if err != nil {
		t.Fatalf("decode after corruption: %v", err)
	}
	assertEqual(t, text, res.Text)
}

func TestDecodeRotated(t *testing.T) {
	cases := []struct {
		text  string
		ecc   ECC
		theta float64 // degrees
	}{
		{"ROTATED QR 12345", ECCMedium, 5},
		{"https://example.com/x", ECCQuartile, -8},
		{"hello rotated world", ECCHigh, 12},
		// Version 7+ symbols take their size from the version information.
		{strings.Repeat("version info ", 12), ECCLow, 7},
		{strings.Repeat("larger symbol 0123456789 ", 30), ECCMedium, -6},
	}
	for _, tc := range cases {
		code := mustEncode(t, tc.text, WithECC(tc.ecc))
		img := mustImage(t, code, WithScale(6), WithQuietZone(6))
		res, err := Decode(rotateGray(img, tc.theta*math.Pi/180))
		if err != nil {
			t.Errorf("v%d @ %.0f°: %v", code.Version(), tc.theta, err)
			continue
		}
		if res.Text != tc.text {
			t.Errorf("v%d @ %.0f°: got %q", code.Version(), tc.theta, res.Text)
		}
	}
}

func TestDecodeInverted(t *testing.T) {
	code := mustEncode(t, "light on dark")
	img := mustImage(t, code, WithForeground(color.White), WithBackground(color.Black))
	assertEqual(t, "light on dark", decodeText(t, img))
	assertEqual(t, "light on dark", decodeText(t, img, WithFastPathOnly()))
	assertEqual(t, "light on dark", decodeText(t, rotateGray(img, 0.1)))
}

func TestDecodeMirrored(t *testing.T) {
	code := mustEncode(t, "seen in a mirror")
	img := mirror(mustImage(t, code, WithScale(4)))
	res, err := Decode(img)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "seen in a mirror", res.Text)
	assertTrue(t, res.Mirrored)
}

func TestDecodeLowContrast(t *testing.T) {
	code := mustEncode(t, "low contrast")
	img := mustImage(t, code,
		WithForeground(color.Gray{Y: 150}),
		WithBackground(color.Gray{Y: 200}))
	assertEqual(t, "low contrast", decodeText(t, img))
}

func TestDecodeUnevenLighting(t *testing.T) {
	code := mustEncode(t, "shadow across the code", WithECC(ECCQuartile))
	src := mustImage(t, code, WithScale(8))
	// Darken the image from left to right so that light modules on the right
	// are darker than dark modules on the left: no global threshold works.
	b := src.Bounds()
	img := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, _, _, _ := src.At(x, y).RGBA()
			base := 40.0 // dark module
			if r > 0x8000 {
				base = 120 // light module
			}
			shade := 1.6 - 1.2*float64(x)/float64(b.Dx())
			img.SetGray(x, y, color.Gray{Y: uint8(min(255, base*shade))})
		}
	}
	if _, err := Decode(img, WithFastPathOnly()); err == nil {
		t.Log("fast path decoded the gradient image; the test is not exercising the adaptive threshold")
	}
	assertEqual(t, "shadow across the code", decodeText(t, img))
}

func TestDecodeTransparentBackground(t *testing.T) {
	code := mustEncode(t, "transparent")
	data, err := code.PNG(WithBackground(color.Transparent))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "transparent", decodeText(t, img))

	// The same with a non-premultiplied image type.
	nrgba := image.NewNRGBA(img.Bounds())
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			nrgba.Set(x, y, img.At(x, y))
		}
	}
	assertEqual(t, "transparent", decodeText(t, nrgba))
}

func TestDecodePalettedPNG(t *testing.T) {
	data, err := mustEncode(t, "paletted round trip").PNG(WithScale(3))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.Paletted); !ok {
		t.Fatalf("decoded PNG is %T, want *image.Paletted", img)
	}
	assertEqual(t, "paletted round trip", decodeText(t, img))
}

func TestDecodeYCbCr(t *testing.T) {
	src := mustImage(t, mustEncode(t, "jpeg-like"), WithScale(4))
	b := src.Bounds()
	img := image.NewYCbCr(b, image.YCbCrSubsampleRatio420)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, _, _, _ := src.At(x, y).RGBA()
			img.Y[img.YOffset(x, y)] = uint8(r >> 8)
		}
	}
	for i := range img.Cb {
		img.Cb[i], img.Cr[i] = 128, 128
	}
	assertEqual(t, "jpeg-like", decodeText(t, img))
}

func TestDecodeECI(t *testing.T) {
	eci := func(n int) Segment {
		s, err := ECISegment(n)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	latin1 := []byte("caf\xe9 na\xefve")                     // "café naïve" in ISO-8859-1
	sjis := []byte{0x93, 0xfa, 0x96, 0x7b, 0x20, 0xb1, 0xb2} // "日本 ｱｲ"

	cases := []struct {
		name string
		segs []Segment
		want string
	}{
		{"utf8 eci", []Segment{eci(26), BytesSegment([]byte("héllo"))}, "héllo"},
		{"latin1 eci", []Segment{eci(3), BytesSegment(latin1)}, "café naïve"},
		{"shift_jis eci", []Segment{eci(20), BytesSegment(sjis)}, "日本 ｱｲ"},
		{"no eci, utf8", []Segment{BytesSegment([]byte("héllo"))}, "héllo"},
		{"no eci, latin1 fallback", []Segment{BytesSegment(latin1)}, "café naïve"},
		{"eci switches charset", []Segment{eci(3), BytesSegment([]byte{0xe9}), eci(26), BytesSegment([]byte("é"))}, "éé"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, err := EncodeSegments(tc.segs)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Decode(mustImage(t, code, WithScale(4)))
			if err != nil {
				t.Fatal(err)
			}
			assertEqual(t, tc.want, res.Text)
		})
	}
}

func TestDecodeECISegmentsReported(t *testing.T) {
	code := mustEncode(t, "héllo", WithUTF8ECI())
	res, err := Decode(mustImage(t, code, WithScale(4)))
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "héllo", res.Text)
	assertEqual(t, ModeECI, res.Segments[0].Mode)
	assertEqual(t, 26, res.Segments[0].ECI)
	for _, s := range res.Segments[1:] {
		assertEqual(t, 26, s.ECI)
	}
}

func TestDecodeUnsupportedECI(t *testing.T) {
	eci, _ := ECISegment(25) // UTF-16BE
	code, err := EncodeSegments([]Segment{eci, BytesSegment([]byte{0, 'a'})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(mustImage(t, code, WithScale(4))); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestDecodeNotFound(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 100, 100))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}
	if _, err := Decode(blank); !errors.Is(err, ErrNotFound) {
		t.Errorf("blank image: error = %v, want ErrNotFound", err)
	}
	if _, err := Decode(image.NewGray(image.Rect(0, 0, 10, 10))); !errors.Is(err, ErrNotFound) {
		t.Errorf("tiny image: error = %v, want ErrNotFound", err)
	}
}

func TestParseBitstreamMalformed(t *testing.T) {
	// pack builds codewords from (value, width) pairs.
	pack := func(fields ...int) []byte {
		var bb bitBuffer
		for i := 0; i < len(fields); i += 2 {
			bb.appendBits(fields[i], fields[i+1])
		}
		return bb.bytes()
	}
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"alphanumeric pair out of range", pack(0x2, 4, 2, 9, 2047, 11), ErrDecodeFailed},
		{"alphanumeric char out of range", pack(0x2, 4, 1, 9, 63, 6), ErrDecodeFailed},
		{"numeric group out of range", pack(0x1, 4, 3, 10, 1023, 10), ErrDecodeFailed},
		{"truncated byte segment", pack(0x4, 4, 200, 8), ErrDecodeFailed},
		{"truncated kanji segment", pack(0x8, 4, 9, 8), ErrDecodeFailed},
		{"invalid ECI designator", pack(0x7, 4, 0xE0, 8), ErrDecodeFailed},
		{"unknown mode", pack(0xF, 4), ErrDecodeFailed},
		{"structured append", pack(0x3, 4), ErrUnsupported},
		{"FNC1", pack(0x5, 4), ErrUnsupported},
		{"hanzi", pack(0xD, 4), ErrUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseBitstream(tc.data, 1); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestShiftJISRoundTrip(t *testing.T) {
	for v := 0; v < len(kanjiToUnicode); v++ {
		if _, ok := kanjiRune(v); !ok {
			continue
		}
		b1, b2 := kanjiToShiftJIS(v)
		got, ok := shiftJISToKanji(b1, b2)
		if !ok || got != v {
			t.Fatalf("value %d -> SJIS %02X%02X -> %d, %v", v, b1, b2, got, ok)
		}
	}
	if _, ok := shiftJISToKanji(0xA0, 0x40); ok {
		t.Error("0xA040 is not a Kanji-mode code")
	}
}

func TestCorrectVersion(t *testing.T) {
	for v := 7; v <= MaxVersion; v++ {
		bits := versionBits(v)
		got, ok := correctVersion(bits ^ 0b100000001000000010) // 3 bit errors
		if !ok || got != v {
			t.Fatalf("version %d with 3 errors: got %d, %v", v, got, ok)
		}
	}
	if _, ok := correctVersion(0); ok {
		t.Error("all-zero bits should not decode to a version")
	}
}

// rotateGray rotates about the center with white fill (nearest-neighbor).
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
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

// mirror flips an image horizontally.
func mirror(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(b.Dx()-1-x, y, src.At(x, y))
		}
	}
	return dst
}

func BenchmarkDecodeClean(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	img, _ := code.Image(WithScale(4))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRotated(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	img, _ := code.Image(WithScale(6))
	rot := rotateGray(img, 0.12)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(rot); err != nil {
			b.Fatal(err)
		}
	}
}

func TestReadVersionNear(t *testing.T) {
	for _, n := range []int{150, 400, 1200} {
		code := mustEncode(t, strings.Repeat("v", n), WithECC(ECCLow))
		img := rotateGray(mustImage(t, code, WithScale(4), WithQuietZone(10)), 0.1)
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		bm := binarizeHybrid(toLuma(img), w, h)
		dark := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && bm[y*w+x] }
		finders, err := findFinders(dark, w, h)
		if err != nil {
			t.Fatal(err)
		}
		tl, tr, bl := orderFinders(finders)
		v, ok := readVersionNear(dark, tl, tr, bl)
		if !ok || v != code.Version() {
			t.Errorf("version %d: readVersionNear = %d, %v", code.Version(), v, ok)
		}
	}
}

func TestDecodeLargeRotated(t *testing.T) {
	text := strings.Repeat("large rotated symbol ", 50)
	code := mustEncode(t, text, WithECC(ECCLow))
	img := rotateGray(mustImage(t, code, WithScale(4), WithQuietZone(12)), -0.09)
	if code.Version() < 20 {
		t.Fatalf("test needs a large symbol, got version %d", code.Version())
	}
	assertEqual(t, text, decodeText(t, img))
}

func TestDecodeIgnoresStrayFinder(t *testing.T) {
	code := mustEncode(t, "stray finder nearby")
	sym := mustImage(t, code, WithScale(4))
	// Paste the symbol on a canvas, slightly rotated so the fast path cannot
	// take it, next to a lone finder pattern at the same scale.
	rot := rotateGray(sym, 0.05)
	b := rot.Bounds()
	canvas := image.NewGray(image.Rect(0, 0, b.Dx()+80, b.Dy()))
	for i := range canvas.Pix {
		canvas.Pix[i] = 255
	}
	for y := 0; y < b.Dy(); y++ {
		copy(canvas.Pix[y*canvas.Stride:], rot.Pix[y*rot.Stride:y*rot.Stride+b.Dx()])
	}
	x0, y0 := b.Dx()+24, b.Dy()/2
	for my := 0; my < 7; my++ {
		for mx := 0; mx < 7; mx++ {
			ring := max(abs(mx-3), abs(my-3))
			if ring == 2 {
				continue // light ring
			}
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 4; dx++ {
					canvas.SetGray(x0+mx*4+dx, y0+my*4+dy, color.Gray{})
				}
			}
		}
	}
	assertEqual(t, "stray finder nearby", decodeText(t, canvas))
}
