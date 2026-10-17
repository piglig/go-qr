package render

import (
	"image/color"
	"io"
	"math/rand"
	"strings"
	"testing"
)

func TestColorToSVG(t *testing.T) {
	assertEqual(t, "#1A2B3C", ColorToSVG(color.RGBA{R: 0x1a, G: 0x2b, B: 0x3c, A: 0xff}))
	assertTrue(t, strings.HasPrefix(ColorToSVG(color.NRGBA{R: 10, G: 20, B: 30, A: 128}), "rgba("))
	assertTrue(t, colorIsTransparent(color.Transparent))
	assertFalse(t, colorIsTransparent(color.Black))
}

func TestGradientAxis(t *testing.T) {
	g := Gradient{Angle: 0}
	x0, y0, dx, dy := g.axis(100)
	assertInDelta(t, 0, x0, 1e-9)
	assertInDelta(t, 50, y0, 1e-9)
	assertInDelta(t, 100, dx, 1e-9)
	assertInDelta(t, 0, dy, 1e-9)

	g.Angle = 45 // corner to corner
	x0, y0, dx, dy = g.axis(100)
	assertInDelta(t, 0, x0, 1e-9)
	assertInDelta(t, 0, y0, 1e-9)
	assertInDelta(t, 100, x0+dx, 1e-9)
	assertInDelta(t, 100, y0+dy, 1e-9)

	a := (&Gradient{From: navy, To: teal, Angle: 0}).ID()
	b := (&Gradient{From: navy, To: teal, Angle: 90}).ID()
	if a == b || !strings.HasPrefix(a, "qr-gradient-") {
		t.Errorf("gradient ids %q and %q", a, b)
	}
}

func TestRoundedCorners(t *testing.T) {
	// A lone dark module has all four corners rounded; one in a horizontal
	// run keeps the corners shared with its neighbors square.
	code := randomSymbol(21, 1)
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

// TestLargeScaleTilesNotCached renders a styled code at a scale whose tiles
// are too large to keep and expects none of them in the cache.
func TestLargeScaleTilesNotCached(t *testing.T) {
	code := randomSymbol(21, 2)
	scale := maxCachedTileScale + 1
	cfg := Config{Scale: scale, FG: color.Black, BG: color.White, ModuleShape: ModuleDot, FinderShape: FinderCircle}
	assertNoError(t, WritePNG(io.Discard, code, &cfg))
	tileCache.Range(func(k, _ any) bool {
		if k.(tileKey).scale == scale {
			t.Errorf("tile %+v cached", k)
		}
		return true
	})
}

func TestLogoBoxParity(t *testing.T) {
	for ver := 1; ver <= 40; ver++ {
		size := 4*ver + 17
		for _, ratio := range []float64{0.01, 0.15, 0.2, 0.33} {
			box := (&Logo{Ratio: ratio}).BoxModules(size)
			if box%2 != size%2 || box < 3 {
				t.Fatalf("version %d ratio %v: box %d modules does not center on a %d-module symbol", ver, ratio, box, size)
			}
		}
	}
}

// randomSymbol returns a symbol of n modules with random modules.
func randomSymbol(n int, seed int64) *Symbol {
	rng := rand.New(rand.NewSource(seed))
	modules := make([][]bool, n)
	for y := range modules {
		modules[y] = make([]bool, n)
		for x := range modules[y] {
			modules[y][x] = rng.Intn(2) == 0
		}
	}
	return &Symbol{N: n, Modules: modules}
}

var (
	navy = color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}
	teal = color.RGBA{G: 0x69, B: 0x5c, A: 0xff}
)
