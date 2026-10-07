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

type badWriter struct{}

func (badWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// moduleAt samples the center of module (mx, my) of a rendered image.
func moduleAt(img image.Image, mx, my, scale, quietZone int) color.Color {
	return img.At((mx+quietZone)*scale+scale/2, (my+quietZone)*scale+scale/2)
}

func sameColor(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// assertRendersModules checks every module of img against code, quiet zone
// included.
func assertRendersModules(t *testing.T, img image.Image, code *Code, scale, quietZone int, fg, bg color.Color) {
	t.Helper()
	side := (code.Size() + 2*quietZone) * scale
	if b := img.Bounds(); b.Dx() != side || b.Dy() != side {
		t.Fatalf("image is %dx%d, want %dx%d", b.Dx(), b.Dy(), side, side)
	}
	for my := -quietZone; my < code.Size()+quietZone; my++ {
		for mx := -quietZone; mx < code.Size()+quietZone; mx++ {
			want := bg
			if code.Module(mx, my) {
				want = fg
			}
			if got := moduleAt(img, mx, my, scale, quietZone); !sameColor(got, want) {
				t.Fatalf("module (%d,%d) = %v, want %v", mx, my, got, want)
			}
		}
	}
}

func TestImage(t *testing.T) {
	code, err := Encode("render me", WithECC(ECCQuartile))
	assertNoError(t, err)

	img, err := code.Image()
	assertNoError(t, err)
	assertRendersModules(t, img, code, defaultScale, defaultQuietZone, color.Black, color.White)

	fg := color.RGBA{R: 0x10, G: 0x20, B: 0x80, A: 0xff}
	bg := color.RGBA{R: 0xf0, G: 0xe0, B: 0xd0, A: 0xff}
	img, err = code.Image(WithScale(3), WithQuietZone(1), WithForeground(fg), WithBackground(bg))
	assertNoError(t, err)
	assertRendersModules(t, img, code, 3, 1, fg, bg)
}

func TestPNG(t *testing.T) {
	code, err := Encode("png output")
	assertNoError(t, err)

	data, err := code.PNG(WithScale(2), WithQuietZone(2))
	assertNoError(t, err)
	img, err := png.Decode(bytes.NewReader(data))
	assertNoError(t, err)
	if _, ok := img.(*image.Paletted); !ok {
		t.Errorf("PNG without logo decoded as %T, want a paletted image", img)
	}
	assertRendersModules(t, img, code, 2, 2, color.Black, color.White)

	var buf bytes.Buffer
	assertNoError(t, code.WritePNG(&buf, WithScale(2), WithQuietZone(2)))
	assertEqual(t, data, buf.Bytes())

	if err := code.WritePNG(badWriter{}); err == nil {
		t.Error("WritePNG to a failing writer returned nil")
	}
}

func TestPNGTransparentBackground(t *testing.T) {
	code, _ := Encode("clear")
	data, err := code.PNG(WithBackground(color.Transparent))
	assertNoError(t, err)
	img, err := png.Decode(bytes.NewReader(data))
	assertNoError(t, err)
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("quiet zone alpha = %d, want 0", a)
	}
}

func TestRenderOptionValidation(t *testing.T) {
	code, _ := Encode("x")
	pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
	tests := []struct {
		name string
		opts []RenderOption
	}{
		{"zero scale", []RenderOption{WithScale(0)}},
		{"negative quiet zone", []RenderOption{WithQuietZone(-1)}},
		{"nil foreground", []RenderOption{WithForeground(nil)}},
		{"nil background", []RenderOption{WithBackground(nil)}},
		{"overflowing size", []RenderOption{WithScale(math.MaxInt32)}},
		{"nil logo", []RenderOption{WithLogo(nil, 0.2)}},
		{"logo ratio 0", []RenderOption{WithLogo(pixel, 0)}},
		{"logo ratio 1", []RenderOption{WithLogo(pixel, 1)}},
		{"logo ratio NaN", []RenderOption{WithLogo(pixel, math.NaN())}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := code.Image(tt.opts...); !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("Image error = %v, want ErrInvalidArgument", err)
			}
			if _, err := code.PNG(tt.opts...); !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("PNG error = %v, want ErrInvalidArgument", err)
			}
			if _, err := code.SVG(tt.opts...); !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("SVG error = %v, want ErrInvalidArgument", err)
			}
		})
	}
	if err := code.WriteText(&bytes.Buffer{}, WithQuietZone(-1)); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("WriteText error = %v, want ErrInvalidArgument", err)
	}
}

func TestSVG(t *testing.T) {
	code, err := Encode("svg output", WithECC(ECCLow))
	assertNoError(t, err)

	data, err := code.SVG(WithScale(3), WithQuietZone(2))
	assertNoError(t, err)
	svg := string(data)
	side := "75" // (21 + 2*2) * 3
	for _, want := range []string{
		`width="` + side + `" height="` + side + `" viewBox="0 0 ` + side + " " + side + `"`,
		`<rect width="` + side + `" height="` + side + `" fill="#FFFFFF"/>`,
		`fill="#000000" fill-rule="evenodd"`,
		`d="M6,6`, // the top-left finder starts at the quiet zone corner
	} {
		assertContains(t, svg, want)
	}
	if strings.HasPrefix(svg, "<?xml") {
		t.Error("XML header present without WithSVGXMLHeader")
	}

	var buf bytes.Buffer
	assertNoError(t, code.WriteSVG(&buf, WithScale(3), WithQuietZone(2)))
	assertEqual(t, data, buf.Bytes())
	if err := code.WriteSVG(badWriter{}); err == nil {
		t.Error("WriteSVG to a failing writer returned nil")
	}
}

func TestSVGTransparentBackgroundOmitsRect(t *testing.T) {
	code, _ := Encode("x")
	data, err := code.SVG(WithBackground(color.Transparent))
	assertNoError(t, err)
	if strings.Contains(string(data), "<rect") {
		t.Error("transparent background still emits a <rect>")
	}
}

func TestColorToSVG(t *testing.T) {
	assertEqual(t, "#1A2B3C", colorToSVG(color.RGBA{R: 0x1a, G: 0x2b, B: 0x3c, A: 0xff}))
	assertTrue(t, strings.HasPrefix(colorToSVG(color.NRGBA{R: 10, G: 20, B: 30, A: 128}), "rgba("))
	assertTrue(t, colorIsTransparent(color.Transparent))
	assertFalse(t, colorIsTransparent(color.Black))
}

func TestWriteText(t *testing.T) {
	code, err := Encode("text", WithECC(ECCLow))
	assertNoError(t, err)

	var buf bytes.Buffer
	assertNoError(t, code.WriteText(&buf, WithQuietZone(1)))
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	side := code.Size() + 2
	if want := (side + 1) / 2; len(lines) != want {
		t.Fatalf("got %d lines, want %d", len(lines), want)
	}
	// Decode the half blocks back into modules and compare.
	for row, line := range lines {
		cells := []rune(line)
		if len(cells) != side {
			t.Fatalf("line %d has %d cells, want %d", row, len(cells), side)
		}
		for col, r := range cells {
			x, y := col-1, 2*row-1
			top := r == '█' || r == '▀'
			bottom := r == '█' || r == '▄'
			if top != code.Module(x, y) || bottom != code.Module(x, y+1) {
				t.Fatalf("cell (%d,%d) = %q does not match modules", col, row, r)
			}
		}
	}

	assertEqual(t, code.text(defaultQuietZone), code.String())
	if err := code.WriteText(badWriter{}); err == nil {
		t.Error("WriteText to a failing writer returned nil")
	}
}

func BenchmarkPNG(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := code.PNG(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkImage(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := code.Image(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSVG(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := code.SVG(); err != nil {
			b.Fatal(err)
		}
	}
}
