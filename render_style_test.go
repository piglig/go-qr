package qr

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
)

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
				if _, _, ok := finderOrigin(x, y, code.Size()); ok {
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

func TestRoundedCorners(t *testing.T) {
	// A lone dark module has all four corners rounded; one in a horizontal
	// run keeps the corners shared with its neighbors square.
	code := mustEncode(t, "x", WithVersionRange(1, 1), WithMask(0))
	for y := 9; y < code.Size()-9; y++ {
		for x := 9; x < code.Size()-9; x++ {
			if !code.Module(x, y) {
				continue
			}
			mask := roundedCorners(code, x, y)
			left, right := code.Module(x-1, y), code.Module(x+1, y)
			if left && mask&(cornerTL|cornerBL) != 0 || right && mask&(cornerTR|cornerBR) != 0 {
				t.Fatalf("module (%d,%d): corner next to a dark neighbor is rounded (mask %04b)", x, y, mask)
			}
		}
	}
	assertTrue(t, insideModule(ModuleRounded, cornerTL, 0.5, 0.5))
	assertFalse(t, insideModule(ModuleRounded, cornerTL, 0.02, 0.02))
	assertTrue(t, insideModule(ModuleRounded, 0, 0.02, 0.02))
	assertFalse(t, insideModule(ModuleDot, 0, 0.05, 0.5))
}

func TestRoundedBox(t *testing.T) {
	circle := roundedBox{0, 0, 7, 3.5}
	assertTrue(t, circle.contains(3.5, 3.5))
	assertTrue(t, circle.contains(0.1, 3.5))
	assertFalse(t, circle.contains(0.3, 0.3))
	square := roundedBox{1, 1, 5, 0}
	assertTrue(t, square.contains(1, 1))
	assertFalse(t, square.contains(6, 3))
}

func TestSVGPathNumbers(t *testing.T) {
	var p svgPath
	p.cmd('M', 4, 1.5)
	p.cmd('L', 1.0/3, -2)
	assertEqual(t, "M4,1.5L0.33,-2", string(p.b))
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

// TestLargeScaleTilesNotCached renders a styled code at a scale whose tiles
// are too large to keep and expects none of them in the cache.
func TestLargeScaleTilesNotCached(t *testing.T) {
	code := mustEncode(t, "x")
	scale := maxCachedTileScale + 1
	_, err := code.PNG(WithScale(scale), WithQuietZone(0), WithModuleShape(ModuleDot), WithFinderShape(FinderCircle))
	assertNoError(t, err)
	tileCache.Range(func(k, _ any) bool {
		if k.(tileKey).scale == scale {
			t.Errorf("tile %+v cached", k)
		}
		return true
	})
}
