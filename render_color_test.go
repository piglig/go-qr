package qr

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

var (
	navy = color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}
	teal = color.RGBA{G: 0x69, B: 0x5c, A: 0xff}
)

func TestColorOptionValidation(t *testing.T) {
	code := mustEncode(t, "x")
	for name, opts := range map[string][]RenderOption{
		"nil ring":       {WithFinderColor(nil, color.Black)},
		"nil center":     {WithFinderColor(color.Black, nil)},
		"nil from":       {WithGradient(nil, color.Black, 0)},
		"nil to":         {WithGradient(color.Black, nil, 0)},
		"NaN angle":      {WithGradient(color.Black, color.Black, math.NaN())},
		"infinite angle": {WithGradient(color.Black, color.Black, math.Inf(1))},
	} {
		if _, err := code.Image(opts...); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: error = %v, want ErrInvalidArgument", name, err)
		}
	}
}

func TestFinderColors(t *testing.T) {
	code := mustEncode(t, "finder colors", WithECC(ECCQuartile))
	img, err := code.Image(WithFinderColor(red, navy), WithScale(10), WithQuietZone(2))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range [3][2]int{{0, 0}, {code.Size() - 7, 0}, {0, code.Size() - 7}} {
		assertTrue(t, sameColor(moduleAt(img, o[0], o[1]+3, 10, 2), red), "ring at %v", o)
		assertTrue(t, sameColor(moduleAt(img, o[0]+3, o[1]+3, 10, 2), navy), "center at %v", o)
	}
	// Data modules keep the foreground.
	for y := 9; y < code.Size()-9; y++ {
		for x := 9; x < code.Size()-9; x++ {
			if code.Module(x, y) {
				assertTrue(t, sameColor(moduleAt(img, x, y, 10, 2), color.Black))
				return
			}
		}
	}
}

func TestGradient(t *testing.T) {
	code := mustEncode(t, "gradient", WithECC(ECCQuartile))
	img, err := code.Image(WithGradient(navy, teal, 0), WithScale(10), WithQuietZone(0))
	if err != nil {
		t.Fatal(err)
	}
	// Left to right: the left finder is near navy, the right one near teal.
	left := rgba(moduleAt(img, 3, 3, 10, 0))
	right := rgba(moduleAt(img, code.Size()-4, 3, 10, 0))
	if absDiff(left.B, navy.B) > 20 || absDiff(right.B, teal.B) > 20 || left.B <= right.B {
		t.Errorf("gradient ends: left %v, right %v", left, right)
	}
	// Finder colors override the gradient.
	img, _ = code.Image(WithGradient(navy, teal, 0), WithFinderColor(red, red), WithScale(10), WithQuietZone(0))
	assertTrue(t, sameColor(moduleAt(img, code.Size()-4, 3, 10, 0), red))
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func TestGradientAxis(t *testing.T) {
	g := gradientConfig{angle: 0}
	x0, y0, dx, dy := g.axis(100)
	assertInDelta(t, 0, x0, 1e-9)
	assertInDelta(t, 50, y0, 1e-9)
	assertInDelta(t, 100, dx, 1e-9)
	assertInDelta(t, 0, dy, 1e-9)

	g.angle = 45 // corner to corner
	x0, y0, dx, dy = g.axis(100)
	assertInDelta(t, 0, x0, 1e-9)
	assertInDelta(t, 0, y0, 1e-9)
	assertInDelta(t, 100, x0+dx, 1e-9)
	assertInDelta(t, 100, y0+dy, 1e-9)

	a := (&gradientConfig{from: navy, to: teal, angle: 0}).id()
	b := (&gradientConfig{from: navy, to: teal, angle: 90}).id()
	if a == b || !strings.HasPrefix(a, "qr-gradient-") {
		t.Errorf("gradient ids %q and %q", a, b)
	}
}

func TestColoredPNG(t *testing.T) {
	code := mustEncode(t, "colored png")
	opts := []RenderOption{WithGradient(navy, teal, 30), WithFinderColor(red, navy)}
	data, err := code.PNG(opts...)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.Paletted); ok {
		t.Fatal("multicolor PNG was written as a paletted image")
	}
	want, _ := code.Image(opts...)
	for y := 0; y < want.Bounds().Dy(); y += 7 {
		for x := 0; x < want.Bounds().Dx(); x += 7 {
			if !sameColor(img.At(x, y), want.At(x, y)) {
				t.Fatalf("pixel (%d,%d) differs from Image", x, y)
			}
		}
	}
}

func TestColoredSVG(t *testing.T) {
	code := mustEncode(t, "hi", WithECC(ECCLow))
	g := &gradientConfig{from: navy, to: teal, angle: 45}

	// A gradient alone keeps the single outline path.
	data, err := code.SVG(WithGradient(navy, teal, 45))
	if err != nil {
		t.Fatal(err)
	}
	svg := string(data)
	assertEqual(t, 1, strings.Count(svg, "<path"))
	assertContains(t, svg, `<linearGradient id="`+g.id()+`" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="290" y2="290">`)
	assertContains(t, svg, `fill="url(#`+g.id()+`)" fill-rule="evenodd"`)

	data, err = code.SVG(WithGradient(navy, teal, 45), WithFinderColor(red, color.Black))
	if err != nil {
		t.Fatal(err)
	}
	svg = string(data)
	assertEqual(t, 3, strings.Count(svg, `fill="#FF0000" fill-rule="evenodd"`))
	assertEqual(t, 3, strings.Count(svg, `fill="#000000"/>`))
	assertEqual(t, 1, strings.Count(svg, `fill="url(#`+g.id()+`)"`))
}

func TestVerifyColors(t *testing.T) {
	code := mustEncode(t, "https://example.com", WithECC(ECCHigh))
	yellow := color.RGBA{R: 0xff, G: 0xeb, B: 0x3b, A: 0xff}
	pink := color.RGBA{R: 0xf8, G: 0xbb, B: 0xd0, A: 0xff}
	for name, tc := range map[string]struct {
		opts []RenderOption
		want error
	}{
		"dark gradient":         {[]RenderOption{WithGradient(navy, teal, 45), WithModuleShape(ModuleRounded)}, nil},
		"finder colors":         {[]RenderOption{WithFinderColor(red, navy), WithFinderShape(FinderCircle)}, nil},
		"gradient to yellow":    {[]RenderOption{WithGradient(navy, yellow, 0)}, ErrUnreadable},
		"pale finder ring":      {[]RenderOption{WithFinderColor(pink, color.Black)}, ErrUnreadable},
		"pale center, gradient": {[]RenderOption{WithGradient(navy, teal, 0), WithFinderColor(color.Black, pink)}, ErrUnreadable},
	} {
		err := code.Verify(tc.opts...)
		if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("%s: Verify = %v, want %v", name, err, tc.want)
		}
	}
}

func BenchmarkColoredPNG(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := code.PNG(WithGradient(navy, teal, 45), WithModuleShape(ModuleRounded)); err != nil {
			b.Fatal(err)
		}
	}
}
