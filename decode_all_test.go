package qr

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"
)

// sheet places the renderings of each text, at scale s, in a grid of
// cols columns on a white canvas, each in a cell of side cell, and returns
// the canvas and the top-left corner of each symbol (quiet zone excluded).
func sheet(t *testing.T, texts []string, cols, cell, s int, opts ...RenderOption) (*image.RGBA, []image.Point) {
	t.Helper()
	rows := (len(texts) + cols - 1) / cols
	canvas := image.NewRGBA(image.Rect(0, 0, cols*cell, rows*cell))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	var at []image.Point
	for i, text := range texts {
		img := mustImage(t, mustEncode(t, text), append([]RenderOption{WithScale(s)}, opts...)...)
		off := image.Pt(i%cols*cell, i/cols*cell)
		draw.Draw(canvas, img.Bounds().Add(off), img, image.Point{}, draw.Src)
		at = append(at, off.Add(image.Pt(defaultQuietZone*s, defaultQuietZone*s)))
	}
	return canvas, at
}

func texts(rs []*DecodeResult) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Text)
	}
	return out
}

func numbered(n int, format string) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf(format, i))
	}
	return out
}

func TestDecodeAllGrid(t *testing.T) {
	want := numbered(9, "symbol %d")
	img, at := sheet(t, want, 3, 230, 6)
	rs, err := DecodeAll(img)
	assertNoError(t, err)
	assertEqual(t, want, texts(rs), "reading order")
	for i, r := range rs {
		// The robust path places corners within a pixel.
		if d := r.Corners[0].Sub(at[i]); abs(d.X) > 1 || abs(d.Y) > 1 {
			t.Errorf("symbol %d top-left at %v, want %v", i, r.Corners[0], at[i])
		}
		assertFalse(t, r.Inverted)
	}
}

func TestDecodeAllRotated(t *testing.T) {
	want := numbered(9, "turned %d")
	img, _ := sheet(t, want, 3, 230, 6)
	// Pad the sheet so that turning it keeps its corners in the image.
	padded := image.NewRGBA(image.Rect(0, 0, 990, 990))
	draw.Draw(padded, padded.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(padded, img.Bounds().Add(image.Pt(150, 150)), img, image.Point{}, draw.Src)
	rs, err := DecodeAll(rotateGray(padded, 0.2))
	assertNoError(t, err)
	assertEqual(t, want, texts(rs))
}

// TestDecodeAllDuplicates expects identical symbols at different places to
// be returned once each.
func TestDecodeAllDuplicates(t *testing.T) {
	img, at := sheet(t, []string{"same", "same", "other"}, 3, 200, 5)
	rs, err := DecodeAll(img)
	assertNoError(t, err)
	assertEqual(t, []string{"same", "same", "other"}, texts(rs))
	assertEqual(t, at[1], rs[1].Corners[0])
}

func TestDecodeAllMixedPolarity(t *testing.T) {
	img, _ := sheet(t, []string{"dark on light"}, 2, 220, 6)
	inv := mustImage(t, mustEncode(t, "light on dark"), WithScale(6), WithForeground(color.White), WithBackground(color.Black))
	draw.Draw(img, inv.Bounds().Add(image.Pt(220, 0)), inv, image.Point{}, draw.Src)
	rs, err := DecodeAll(img)
	assertNoError(t, err)
	assertEqual(t, []string{"dark on light", "light on dark"}, texts(rs))
	assertFalse(t, rs[0].Inverted)
	assertTrue(t, rs[1].Inverted)
}

// TestDecodeAllStructuredAppend reads a whole sequence from one image, in
// any order, and joins it.
func TestDecodeAllStructuredAppend(t *testing.T) {
	text := "A message long enough to be split into several QR Code symbols, which a single photo then captures together."
	parts, err := EncodeStructured(text, WithVersionRange(1, 2), WithECC(ECCLow))
	assertNoError(t, err)
	assertTrue(t, len(parts) > 2, "%d parts", len(parts))
	canvas := image.NewRGBA(image.Rect(0, 0, len(parts)*200, 200))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	for i := range parts {
		img := mustImage(t, parts[len(parts)-1-i], WithScale(5)) // reversed
		draw.Draw(canvas, img.Bounds().Add(image.Pt(i*200, 0)), img, image.Point{}, draw.Src)
	}
	rs, err := DecodeAll(canvas)
	assertNoError(t, err)
	assertLen(t, rs, len(parts))
	joined, err := JoinStructuredAppend(rs...)
	assertNoError(t, err)
	assertEqual(t, text, joined)
}

// TestDecodeAllSingle expects DecodeAll to read what Decode reads in
// images of one symbol.
func TestDecodeAllSingle(t *testing.T) {
	code := mustEncode(t, "only one")
	for name, img := range map[string]image.Image{
		"crisp":   mustImage(t, code, WithScale(4)),
		"rotated": rotateGray(mustImage(t, code, WithScale(5)), 0.3),
		"photo":   benchPhoto(1200, 900, true),
	} {
		want, err := Decode(img)
		assertNoError(t, err, name)
		rs, err := DecodeAll(img)
		assertNoError(t, err, name)
		assertLen(t, rs, 1, name)
		assertEqual(t, want.Text, rs[0].Text, name)
		assertEqual(t, want.Corners, rs[0].Corners, name)
	}
}

func TestDecodeAllImageBounds(t *testing.T) {
	img, at := sheet(t, []string{"a", "b"}, 2, 200, 5)
	sub := img.SubImage(image.Rect(10, 0, 400, 200))
	rs, err := DecodeAll(sub)
	assertNoError(t, err)
	assertEqual(t, []string{"a", "b"}, texts(rs))
	assertEqual(t, at[0], rs[0].Corners[0])
}

func TestDecodeAllMaxSymbols(t *testing.T) {
	img, _ := sheet(t, numbered(6, "n%d"), 3, 200, 5)
	rs, err := DecodeAll(img, WithMaxSymbols(2))
	assertNoError(t, err)
	assertLen(t, rs, 2)
	for _, n := range []int{0, -1} {
		if _, err := DecodeAll(img, WithMaxSymbols(n)); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("WithMaxSymbols(%d): error %v, want ErrInvalidArgument", n, err)
		}
	}
	// Decode ignores it.
	_, err = Decode(img, WithMaxSymbols(0))
	assertNoError(t, err)
}

// TestDecodeAllErrors expects the error Decode returns when nothing
// decodes.
func TestDecodeAllErrors(t *testing.T) {
	for name, img := range map[string]image.Image{
		"blank":   image.NewGray(image.Rect(0, 0, 100, 100)),
		"tiny":    image.NewGray(image.Rect(0, 0, 10, 10)),
		"texture": benchPhoto(320, 240, false),
	} {
		_, want := Decode(img)
		rs, err := DecodeAll(img)
		assertNil(t, rs, name)
		if err == nil || errors.Is(want, ErrNotFound) != errors.Is(err, ErrNotFound) || errors.Is(want, ErrDecodeFailed) != errors.Is(err, ErrDecodeFailed) {
			t.Errorf("%s: error %v, want like %v", name, err, want)
		}
	}
	// The fast path alone reads one crisp symbol.
	code := mustEncode(t, "fast")
	rs, err := DecodeAll(mustImage(t, code), WithFastPathOnly())
	assertNoError(t, err)
	assertEqual(t, []string{"fast"}, texts(rs))
}

// TestDecodeAllStyled reads several symbols with round finders and dots.
func TestDecodeAllStyled(t *testing.T) {
	for _, fs := range []FinderShape{FinderRounded, FinderCircle} {
		want := numbered(6, "styled %d")
		img, _ := sheet(t, want, 3, 230, 6, WithFinderShape(fs), WithModuleShape(ModuleDot))
		rs, err := DecodeAll(img)
		assertNoError(t, err)
		assertEqual(t, want, texts(rs), "finder shape %v", fs)
	}
}

// TestDecodeAllFinderTiles bounds the time DecodeAll takes on an image
// tiled with finder patterns, which hold hundreds of thousands of
// candidates.
func TestDecodeAllFinderTiles(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test")
	}
	start := time.Now()
	if _, err := DecodeAll(finderTiles(3200, 3200, 1)); err == nil {
		t.Fatal("decoded an image without a code")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("DecodeAll took %v", d)
	}
}

func TestReadingOrder(t *testing.T) {
	box := func(x, y int) *DecodeResult {
		return &DecodeResult{Text: fmt.Sprint(x, ",", y), Corners: [4]image.Point{{x, y}, {x + 100, y}, {x + 100, y + 100}, {x, y + 100}}}
	}
	// A row whose symbols sit a little higher or lower stays one row.
	rs := readingOrder([]*DecodeResult{box(300, 10), box(0, 30), box(150, 0), box(0, 200), box(160, 190)})
	assertEqual(t, []string{"0,30", "150,0", "300,10", "0,200", "160,190"}, texts(rs))
}

func TestInQuad(t *testing.T) {
	q := [4]image.Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	mirrored := [4]image.Point{{0, 0}, {0, 10}, {10, 10}, {10, 0}}
	for _, quad := range [][4]image.Point{q, mirrored} {
		assertTrue(t, inQuad(quad, 5, 5))
		assertTrue(t, inQuad(quad, 0, 3))
		assertFalse(t, inQuad(quad, 11, 5))
		assertFalse(t, inQuad(quad, -1, -1))
	}
}

func BenchmarkDecodeAll(b *testing.B) {
	// A 3×3 sheet with a margin, so that turning it keeps every symbol.
	canvas := image.NewRGBA(image.Rect(0, 0, 900, 900))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	for i := 0; i < 9; i++ {
		code, _ := Encode(fmt.Sprintf("https://example.com/item/%d", i))
		img, _ := code.Image(WithScale(5))
		draw.Draw(canvas, img.Bounds().Add(image.Pt(105+i%3*230, 105+i/3*230)), img, image.Point{}, draw.Src)
	}
	img := rotateGray(canvas, 0.1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if rs, err := DecodeAll(img); err != nil || len(rs) != 9 {
			b.Fatalf("decoded %d symbols: %v", len(rs), err)
		}
	}
}

func BenchmarkDecodeAllNoCode(b *testing.B) {
	img := benchPhoto(640, 480, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeAll(img); err == nil {
			b.Fatal("decoded an image without a code")
		}
	}
}
