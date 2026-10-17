package qr

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"

	"github.com/piglig/go-qr/v2/internal/detect"
	"github.com/piglig/go-qr/v2/internal/symbol"
)

func glareImage(code *Code, scale int, cx, cy, radius, strength, sigma float64) *image.Gray {
	src, _ := code.Image(WithScale(scale))
	b := src.Bounds()
	g := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			v := float64(src.RGBAAt(x, y).G)
			d := math.Hypot(float64(x)-cx, float64(y)-cy) / radius
			k := strength * math.Exp(-d*d)
			g.SetGray(x, y, color.Gray{Y: uint8(v + (255-v)*k)})
		}
	}
	return blurGray(g, sigma)
}

func blurGray(g *image.Gray, sigma float64) *image.Gray {
	if sigma <= 0 {
		return g
	}
	r := int(math.Ceil(3 * sigma))
	k := make([]float64, 2*r+1)
	s := 0.0
	for i := range k {
		k[i] = math.Exp(-float64((i-r)*(i-r)) / (2 * sigma * sigma))
		s += k[i]
	}
	b := g.Bounds()
	tmp := make([]float64, b.Dx()*b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			a := 0.0
			for i := -r; i <= r; i++ {
				xx := min(max(x+i, 0), b.Dx()-1)
				a += k[i+r] * float64(g.Pix[y*g.Stride+xx])
			}
			tmp[y*b.Dx()+x] = a / s
		}
	}
	out := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			a := 0.0
			for i := -r; i <= r; i++ {
				yy := min(max(y+i, 0), b.Dy()-1)
				a += k[i+r] * tmp[yy*b.Dx()+x]
			}
			out.Pix[y*out.Stride+x] = uint8(math.Round(a / s))
		}
	}
	return out
}

// hardOnly reads grids by hard decisions only, as Verify does.
func hardOnly(grid [][]bool, rel []float32) (*DecodeResult, error) {
	if rel != nil {
		return nil, ErrDecodeFailed
	}
	return decodeGrid(grid, nil)
}

func TestDecodeGlareSoftly(t *testing.T) {
	// A glare spot washes out part of a version 2-L code and blur smears
	// it: hard decisions leave too many errors, soft ones do not.
	text := "https://example.com/glare"
	code, err := Encode(text, WithECC(ECCLow), WithoutECCBoost())
	assertNoError(t, err)
	assertEqual(t, 2, code.Version())
	side := float64((code.Size() + 8) * 6)
	img := glareImage(code, 6, side*0.55, side*0.5, side*0.16, 0.85, 2.5)
	res, err := Decode(img)
	assertNoError(t, err)
	assertEqual(t, text, res.Text)
	if _, err := searchImage(img, decodeConfig{}, hardOnly); err == nil {
		t.Error("hard decisions alone read the image; the test no longer exercises soft decoding")
	}
}

func TestSoftDecodingRejectsRandomGrids(t *testing.T) {
	// Grids whose function patterns are intact pass softGate; with random
	// data modules, no attempt may decode. 3 million such grids decoded
	// none while the policy was developed.
	rng := rand.New(rand.NewSource(1))
	n := 1500
	if testing.Short() {
		n = 200
	}
	passed := 0
	for i := 0; i < n; i++ {
		ver := 1 + rng.Intn(6)
		dim := 4*ver + 17
		b := symbol.NewBuilder(ver, symbol.Level(rng.Intn(4)))
		b.DrawFunctionPatterns()
		b.DrawFormatBits(rng.Intn(8))
		vals := make([]float64, dim*dim)
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				dark := b.Modules[y][x]
				if !b.IsFunction[y][x] {
					dark = rng.Intn(2) == 0
				}
				v := 200.0
				if dark {
					v = 60
				}
				vals[y*dim+x] = v + rng.NormFloat64()*25
			}
		}
		if pilotAccuracy(detect.ThresholdModules(vals, dim, false)) >= softGate {
			passed++
		}
		if res, err := readSoft(vals, dim, false, decodeGrid); err == nil {
			t.Fatalf("grid %d (version %d) decoded to %q", i, ver, res.Text)
		}
	}
	if passed < n*9/10 {
		t.Errorf("only %d of %d grids passed the gate", passed, n)
	}
}

func TestPilotAccuracy(t *testing.T) {
	code := mustEncode(t, "pilots", WithVersionRange(7, 7))
	assertEqual(t, 1.0, pilotAccuracy(code.modules))
	inv := make([][]bool, code.size)
	for y := range inv {
		inv[y] = make([]bool, code.size)
		for x := range inv[y] {
			inv[y][x] = !code.modules[y][x]
		}
	}
	assertEqual(t, 0.0, pilotAccuracy(inv))
	if a := pilotAccuracy(transpose(code.modules)); a != 1 {
		t.Errorf("mirror image: %v", a)
	}
	if a := pilotAccuracy(make([][]bool, 22)); a != 0 {
		t.Errorf("no symbol size: %v", a)
	}
}

func TestHasData(t *testing.T) {
	if hasData(nil) || hasData(&DecodeResult{}) || hasData(&DecodeResult{Segments: []DecodedSegment{{Mode: ModeECI}, {Mode: ModeByte}}}) {
		t.Error("no data segment counted as data")
	}
	if !hasData(&DecodeResult{Segments: []DecodedSegment{{Mode: ModeNumeric, NumChars: 1}}}) {
		t.Error("data segment not counted")
	}
}

func TestClassifyModulesUndoesBlurAndShade(t *testing.T) {
	// Luminances with blur leaking from the neighbors and light falling
	// off across the symbol, as in a photo: the channel model, fitted to
	// the function patterns, decides more modules right than the local
	// threshold.
	code := mustEncode(t, "classify the modules of a blurred and shaded code", WithECC(ECCLow))
	dim := code.size
	rng := rand.New(rand.NewSource(1))
	vals := make([]float64, dim*dim)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			around := 0.0
			for _, d := range detect.NeighborWeights {
				if code.Module(x+d[1], y+d[0]) {
					around += float64(d[2])
				}
			}
			light := 220 - 120*float64(x)/float64(dim)
			v := light - 80*around/12
			if code.modules[y][x] {
				v -= 0.35 * light
			}
			vals[y*dim+x] = v + rng.NormFloat64()*6
		}
	}
	wrong := func(g [][]bool) int {
		n := 0
		for y := range g {
			for x := range g[y] {
				if g[y][x] != code.modules[y][x] {
					n++
				}
			}
		}
		return n
	}
	eq, rel := detect.ClassifyModules(vals, dim, false)
	th := wrong(detect.ThresholdModules(vals, dim, false))
	t.Logf("modules wrong: threshold %d, channel model %d", th, wrong(eq))
	// 38 and 3 when written.
	if th < 20 || wrong(eq) > th/4 {
		t.Errorf("channel model %d modules wrong, threshold %d", wrong(eq), th)
	}
	if len(rel) != dim*dim {
		t.Fatalf("%d reliabilities", len(rel))
	}
	if g, r := detect.ClassifyModules(vals[:100], 10, false); g != nil || r != nil {
		t.Error("classified a grid of no symbol size")
	}
}

func TestSoftAttemptsLeaveProtectionUnused(t *testing.T) {
	// Three wrong codewords in a version 1-L symbol: hard decisions correct
	// them, using the check bytes Table 9 reserves; a soft attempt must not,
	// even with the reliabilities pointing at exactly those codewords.
	code := mustEncode(t, "protection", WithECC(ECCLow), WithVersionRange(1, 1), WithoutECCBoost())
	n := code.size
	grid := make([][]bool, n)
	for y := range grid {
		grid[y] = append([]bool(nil), code.modules[y]...)
	}
	rel := make([]float32, n*n)
	for i := range rel {
		rel[i] = 1
	}
	bitAt := symbol.BitAt(1)
	flipped := map[int]bool{}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			b := bitAt[y][x]
			if b < 0 {
				continue
			}
			if cw := int(b) / 8; cw == 2 || cw == 9 || cw == 17 {
				rel[y*n+x] = 0.01
				if !flipped[cw] {
					grid[y][x] = !grid[y][x]
					flipped[cw] = true
				}
			}
		}
	}
	res, err := decodeModules(grid, nil)
	assertNoError(t, err)
	assertEqual(t, "protection", res.Text)
	if res, err := decodeModules(grid, rel); err == nil {
		t.Errorf("soft attempt corrected 3 errors of a 1-L symbol: %q", res.Text)
	}
}

// TestSoftDecodingBuildsNoTemplates expects the attempts on grids of any
// version to leave the encoder's templates, which hold far more per version
// than decoding needs, unbuilt.
func TestSoftDecodingBuildsNoTemplates(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	tried := 0
	for _, ver := range []int{27, 33, 39} {
		if symbol.HasTemplate(ver) {
			continue // built by an encoding test
		}
		tried++
		dim := 4*ver + 17
		b := symbol.NewBuilder(ver, symbol.Medium)
		b.DrawFunctionPatterns()
		b.DrawFormatBits(rng.Intn(8))
		vals := make([]float64, dim*dim)
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				dark := b.Modules[y][x]
				if !b.IsFunction[y][x] {
					dark = rng.Intn(2) == 0
				}
				v := 200.0
				if dark {
					v = 60
				}
				vals[y*dim+x] = v + rng.NormFloat64()*25
			}
		}
		assertTrue(t, pilotAccuracy(detect.ThresholdModules(vals, dim, false)) >= softGate, "version %d grid fails the gate", ver)
		if _, err := readSoft(vals, dim, false, decodeGrid); err == nil {
			t.Fatalf("random version %d grid decoded", ver)
		}
		if symbol.HasTemplate(ver) {
			t.Errorf("soft decoding built the version %d template", ver)
		}
	}
	if tried == 0 {
		t.Skip("every version tried has a template from another test")
	}
}
