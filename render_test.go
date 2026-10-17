package qr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/piglig/go-qr/v2/internal/render"
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
		{"empty logo", []RenderOption{WithLogo(image.NewRGBA(image.Rectangle{}), 0.2)}},
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

// TestRasterSizeLimit expects PNG and Image to refuse images over
// maxRasterSide pixels on a side, which would allocate gigabytes, while SVG,
// whose scale is in user units, still renders.
func TestRasterSizeLimit(t *testing.T) {
	code := mustEncode(t, "x")
	big := []RenderOption{WithScale(1000)} // 29000 pixels
	if _, err := code.Image(big...); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("Image error = %v, want ErrInvalidArgument", err)
	}
	for _, opts := range [][]RenderOption{big, append(big, WithModuleShape(ModuleDot)), append(big, WithGradient(color.Black, color.Black, 0))} {
		if _, err := code.PNG(opts...); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("PNG error = %v, want ErrInvalidArgument", err)
		}
	}
	_, err := code.SVG(big...)
	assertNoError(t, err)

	// (21 + 2·54) · 127 = 16383 pixels, the largest side below the limit
	// that a symbol of version 1 can have.
	for _, tt := range []struct {
		scale int
		ok    bool
	}{{127, true}, {128, false}} {
		cfg, err := newRenderConfig([]RenderOption{WithQuietZone(54), WithScale(tt.scale)})
		assertNoError(t, err)
		sym := code.symbol()
		_, err = cfg.RasterSide(&sym)
		assertEqual(t, tt.ok, err == nil, "scale %d: %v", tt.scale, err)
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

// rgba converts c to 8-bit alpha-premultiplied RGBA.
func rgba(c color.Color) color.RGBA {
	return color.RGBAModel.Convert(c).(color.RGBA)
}

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
	g := &render.Gradient{From: navy, To: teal, Angle: 45}

	// A gradient alone keeps the single outline path.
	data, err := code.SVG(WithGradient(navy, teal, 45))
	if err != nil {
		t.Fatal(err)
	}
	svg := string(data)
	assertEqual(t, 1, strings.Count(svg, "<path"))
	assertContains(t, svg, `<linearGradient id="`+g.ID()+`" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="290" y2="290">`)
	assertContains(t, svg, `fill="url(#`+g.ID()+`)" fill-rule="evenodd"`)

	data, err = code.SVG(WithGradient(navy, teal, 45), WithFinderColor(red, color.Black))
	if err != nil {
		t.Fatal(err)
	}
	svg = string(data)
	assertEqual(t, 3, strings.Count(svg, `fill="#FF0000" fill-rule="evenodd"`))
	assertEqual(t, 3, strings.Count(svg, `fill="#000000"/>`))
	assertEqual(t, 1, strings.Count(svg, `fill="url(#`+g.ID()+`)"`))
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

// paintPaletted renders code into a two-color paletted image. png.Encode of
// it is the reference writeTwoColorPNG matches byte for byte.
func paintPaletted(code *Code, c *renderConfig, side int) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, side, side), color.Palette{c.BG, c.FG})
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			if code.Module(x/c.Scale-c.QuietZone, y/c.Scale-c.QuietZone) {
				img.Pix[y*img.Stride+x] = 1
			}
		}
	}
	return img
}

// referencePNG is the PNG png.Encode writes for code with opts.
func referencePNG(t *testing.T, code *Code, opts ...RenderOption) []byte {
	t.Helper()
	cfg, err := newRenderConfig(opts)
	assertNoError(t, err)
	sym := code.symbol()
	side, err := cfg.RasterSide(&sym)
	assertNoError(t, err)
	var buf bytes.Buffer
	assertNoError(t, png.Encode(&buf, paintPaletted(code, &cfg, side)))
	return buf.Bytes()
}

// idatChunks counts the IDAT chunks of a PNG.
func idatChunks(t *testing.T, b []byte) int {
	t.Helper()
	n := 0
	for i := 8; i+8 <= len(b); {
		length := int(binary.BigEndian.Uint32(b[i:]))
		if string(b[i+4:i+8]) == "IDAT" {
			n++
		}
		i += 12 + length
	}
	return n
}

func TestPNGMatchesStandardEncoder(t *testing.T) {
	small, _ := Encode("https://github.com/piglig/go-qr")
	large, _ := EncodeBytes(bytes.Repeat([]byte{0x5a, 0xc3, 0x17}, 900), WithECC(ECCLow), WithVersionRange(40, 40))
	gray := color.Gray16{Y: 0x8000}
	tests := map[string]struct {
		code *Code
		opts []RenderOption
	}{
		"default":             {small, nil},
		"scale 1":             {small, []RenderOption{WithScale(1)}},
		"scale 3":             {small, []RenderOption{WithScale(3)}},
		"scale 8":             {small, []RenderOption{WithScale(8)}},
		"scale 13":            {small, []RenderOption{WithScale(13), WithQuietZone(1)}},
		"no quiet zone":       {small, []RenderOption{WithQuietZone(0), WithScale(5)}},
		"wide quiet zone":     {small, []RenderOption{WithQuietZone(11), WithScale(2)}},
		"colors":              {small, []RenderOption{WithForeground(color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}), WithBackground(color.NRGBA{R: 0xff, G: 0xf8, B: 0xe1, A: 0xff})}},
		"16-bit colors":       {small, []RenderOption{WithForeground(gray), WithBackground(color.White)}},
		"same colors":         {small, []RenderOption{WithForeground(color.Black), WithBackground(color.Black)}},
		"transparent bg":      {small, []RenderOption{WithBackground(color.Transparent)}},
		"translucent fg":      {small, []RenderOption{WithForeground(color.NRGBA{A: 0x80})}},
		"both translucent":    {small, []RenderOption{WithForeground(color.NRGBA{R: 9, A: 0x40}), WithBackground(color.NRGBA{G: 9, A: 0xc0})}},
		"version 40":          {large, []RenderOption{WithScale(9)}},
		"several IDAT chunks": {large, []RenderOption{WithScale(37)}},
		"version 40 scale 1":  {large, []RenderOption{WithScale(1), WithQuietZone(0)}},
	}
	multipleIDAT := false
	for name, tc := range tests {
		got, err := tc.code.PNG(tc.opts...)
		assertNoError(t, err)
		want := referencePNG(t, tc.code, tc.opts...)
		if !bytes.Equal(got, want) {
			t.Errorf("%s: PNG differs from png.Encode: %d bytes, want %d", name, len(got), len(want))
		}
		if idatChunks(t, want) > 1 {
			multipleIDAT = true
		}
	}
	if large.Version() != 40 {
		t.Errorf("large code is version %d, want 40", large.Version())
	}
	if !multipleIDAT {
		t.Error("no test case splits its data into several IDAT chunks")
	}
}

func TestPNGConcurrent(t *testing.T) {
	code, _ := Encode("concurrent")
	want := referencePNG(t, code)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if got, err := code.PNG(); err != nil || !bytes.Equal(got, want) {
					t.Errorf("concurrent PNG differs or failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// limitWriter fails once n bytes have been written.
type limitWriter struct{ n int }

func (w *limitWriter) Write(b []byte) (int, error) {
	if len(b) > w.n {
		k := w.n
		w.n = 0
		return k, errors.New("disk full")
	}
	w.n -= len(b)
	return len(b), nil
}

func TestWritePNGWriterErrors(t *testing.T) {
	code, _ := Encode("https://github.com/piglig/go-qr")
	full, _ := code.PNG()
	for n := 0; n < len(full); n++ {
		err := code.WritePNG(&limitWriter{n: n})
		if err == nil || !strings.Contains(err.Error(), "disk full") {
			t.Fatalf("writer failing after %d of %d bytes: error = %v", n, len(full), err)
		}
	}
	// The writer pool still produces correct output after failed writes.
	again, _ := code.PNG()
	if !bytes.Equal(again, full) {
		t.Error("PNG differs after failed writes")
	}
}

var allStyles = func() [][]RenderOption {
	var out [][]RenderOption
	for _, ms := range []ModuleShape{ModuleSquare, ModuleDot, ModuleRounded} {
		for _, fs := range []FinderShape{FinderSquare, FinderRounded, FinderCircle} {
			out = append(out, []RenderOption{WithModuleShape(ms), WithFinderShape(fs)})
		}
	}
	return out
}()

func TestStyleOptionValidation(t *testing.T) {
	code := mustEncode(t, "x")
	for _, opts := range [][]RenderOption{
		{WithModuleShape(ModuleShape(9))},
		{WithModuleShape(ModuleShape(-1))},
		{WithFinderShape(FinderShape(9))},
	} {
		if _, err := code.PNG(opts...); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("PNG error = %v, want ErrInvalidArgument", err)
		}
		if _, err := code.SVG(opts...); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("SVG error = %v, want ErrInvalidArgument", err)
		}
	}
}

// TestStyledModuleCenters checks that every module center keeps its color
// in every style, since that is what readers sample.
func TestStyledModuleCenters(t *testing.T) {
	code := mustEncode(t, "styled centers 0123456789", WithECC(ECCQuartile))
	for i, opts := range allStyles {
		img, err := code.Image(append(opts, WithScale(10), WithQuietZone(2))...)
		if err != nil {
			t.Fatal(err)
		}
		isDark := func(x, y int) bool {
			r, _, _, _ := moduleAt(img, x, y, 10, 2).RGBA()
			return r < 0x8000
		}
		for y := -2; y < code.Size()+2; y++ {
			for x := -2; x < code.Size()+2; x++ {
				if _, _, ok := render.FinderOrigin(x, y, code.Size()); ok {
					continue // corners of round finders are light by design
				}
				if isDark(x, y) != code.Module(x, y) {
					t.Fatalf("style %d: module (%d,%d) center dark=%v, want %v", i, x, y, isDark(x, y), code.Module(x, y))
				}
			}
		}
		// The row and column through each finder center read 1:1:3:1:1.
		for _, o := range [3][2]int{{0, 0}, {code.Size() - 7, 0}, {0, code.Size() - 7}} {
			for k, want := range []bool{true, false, true, true, true, false, true} {
				if isDark(o[0]+k, o[1]+3) != want || isDark(o[0]+3, o[1]+k) != want {
					t.Fatalf("style %d: finder at %v is not 1:1:3:1:1 through its center", i, o)
				}
			}
		}
	}
}

func TestStyledAntiAliasing(t *testing.T) {
	code := mustEncode(t, "anti-aliased")
	img, err := code.Image(WithModuleShape(ModuleDot), WithScale(10))
	if err != nil {
		t.Fatal(err)
	}
	gray := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if v := img.Pix[i]; v > 0 && v < 255 {
			gray++
		}
	}
	if gray == 0 {
		t.Fatal("dot edges are not anti-aliased")
	}
}

func TestStyledPNG(t *testing.T) {
	code := mustEncode(t, "x")
	data, err := code.PNG(WithModuleShape(ModuleDot))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// Anti-aliased edges need the blended colors of a 256-entry palette.
	if p, ok := img.(*image.Paletted); !ok || len(p.Palette) != 256 {
		t.Fatalf("styled PNG decoded as %T, want a 256-color paletted image", img)
	}
	// It matches Image pixel for pixel.
	want, _ := code.Image(WithModuleShape(ModuleDot))
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			if !sameColor(img.At(x, y), want.At(x, y)) {
				t.Fatalf("pixel (%d,%d) = %v, Image has %v", x, y, img.At(x, y), want.At(x, y))
			}
		}
	}
}

// TestStylesAreReadable renders every style combination at several
// versions, ECC levels and scales and decodes it.
func TestStylesAreReadable(t *testing.T) {
	for i, opts := range allStyles {
		for _, ecc := range []ECC{ECCLow, ECCHigh} {
			for _, ver := range []int{1, 7, 25, 40} {
				code := mustEncode(t, "styled", WithECC(ecc), WithoutECCBoost(), WithVersionRange(ver, ver))
				for _, scale := range []int{3, 8} {
					if err := code.Verify(append(opts, WithScale(scale))...); err != nil {
						t.Errorf("style %d version %d %v scale %d: %v", i, ver, ecc, scale, err)
					}
				}
			}
		}
	}
}

func TestStyledSVG(t *testing.T) {
	code := mustEncode(t, "hi", WithECC(ECCLow))
	plain, _ := code.SVG()
	square, _ := code.SVG(WithModuleShape(ModuleSquare), WithFinderShape(FinderSquare))
	assertEqual(t, plain, square)

	data, err := code.SVG(WithModuleShape(ModuleDot), WithFinderShape(FinderCircle), WithScale(10))
	if err != nil {
		t.Fatal(err)
	}
	svg := string(data)
	// One data path plus a ring and a center per finder.
	assertEqual(t, 7, strings.Count(svg, "<path"))
	assertEqual(t, 3, strings.Count(svg, `fill-rule="evenodd"`))
	assertContains(t, svg, "a4,4 0 1,0 8,0") // a dot of radius 0.4 module
	assertContains(t, svg, "a35,35 0 0,1")   // the circular finder ring
}

func BenchmarkStyledPNG(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := code.PNG(WithModuleShape(ModuleRounded), WithFinderShape(FinderRounded)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStyledSVG(b *testing.B) {
	code, _ := Encode("https://github.com/piglig/go-qr?ref=bench&v=1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := code.SVG(WithModuleShape(ModuleRounded), WithFinderShape(FinderRounded)); err != nil {
			b.Fatal(err)
		}
	}
}
