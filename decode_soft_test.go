package qr

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"

	"github.com/piglig/go-qr/v2/internal/reedsolomon"
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
		b := newBuilder(ver, ECC(rng.Intn(4)))
		b.drawFunctionPatterns()
		b.drawFormatBits(rng.Intn(8))
		vals := make([]float64, dim*dim)
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				dark := b.modules[y][x]
				if !b.isFunction[y][x] {
					dark = rng.Intn(2) == 0
				}
				v := 200.0
				if dark {
					v = 60
				}
				vals[y*dim+x] = v + rng.NormFloat64()*25
			}
		}
		if pilotAccuracy(thresholdModules(vals, dim, false)) >= softGate {
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

func TestSoftCorrectKeepsProtection(t *testing.T) {
	// A version 1-L block has 7 check bytes, of which Table 9 reserves 3:
	// hard decisions correct 3 errors, soft attempts no more than 2 even
	// with every reliability pointing at them.
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		data := make([]byte, 19)
		rng.Read(data)
		div, _ := reedsolomon.Divisor(7)
		orig := append(data, reedsolomon.Remainder(data, div)...)
		cw := append([]byte(nil), orig...)
		pos := rng.Perm(len(cw))[:3]
		rel := make([]float32, len(cw))
		for j := range rel {
			rel[j] = 1
		}
		for _, p := range pos {
			cw[p] ^= byte(1 + rng.Intn(255))
			rel[p] = 0
		}
		budget := 7 - misdecodeProtection(1, ECCLow) - gmdMargin
		if _, err := softCorrect(append([]byte(nil), cw...), 7, rel, budget); err == nil {
			t.Fatalf("block %d: three errors corrected within budget %d", i, budget)
		}
		if err := reedsolomon.Correct(cw, 7); err != nil {
			t.Fatalf("block %d: hard decisions: %v", i, err)
		}
	}
	// Two errors at the least reliable bytes are corrected by erasing them.
	data := []byte("soft decision deco")
	div, _ := reedsolomon.Divisor(10)
	orig := append(append([]byte(nil), data...), reedsolomon.Remainder(data, div)...)
	cw := append([]byte(nil), orig...)
	rel := make([]float32, len(cw))
	for j := range rel {
		rel[j] = 1
	}
	for _, p := range []int{2, 5, 11, 20} {
		cw[p] ^= 0x5a
		rel[p] = 0.01
	}
	if erased, err := softCorrect(cw, 10, rel, 8); err != nil || string(cw) != string(orig) || erased != nil {
		t.Errorf("four errors within the budget: erased %v, %v", erased, err)
	}
	// Five are not, by hard decisions: two of them are erased.
	cw = append([]byte(nil), orig...)
	for _, p := range []int{2, 5, 11, 20, 23} {
		cw[p] ^= 0x5a
		rel[p] = 0.01
	}
	erased, err := softCorrect(cw, 10, rel, 8)
	if err != nil || string(cw) != string(orig) {
		t.Errorf("five errors at the least reliable bytes: %v", err)
	}
	if len(erased) != 2 || rel[erased[0]] != 0.01 || rel[erased[1]] != 0.01 {
		t.Errorf("erased %v, want two of the least reliable bytes", erased)
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
			for _, d := range neighborWeights {
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
	eq, rel := classifyModules(vals, dim, false)
	th := wrong(thresholdModules(vals, dim, false))
	t.Logf("modules wrong: threshold %d, channel model %d", th, wrong(eq))
	// 38 and 3 when written.
	if th < 20 || wrong(eq) > th/4 {
		t.Errorf("channel model %d modules wrong, threshold %d", wrong(eq), th)
	}
	if len(rel) != dim*dim {
		t.Fatalf("%d reliabilities", len(rel))
	}
	if g, r := classifyModules(vals[:100], 10, false); g != nil || r != nil {
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
	bitAt := getTemplate(1).bitAt
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

func TestPilotsAreTheKnownModules(t *testing.T) {
	for ver := MinVersion; ver <= MaxVersion; ver++ {
		b := newBuilder(ver, ECCLow)
		b.drawFunctionPatterns()
		known := map[[2]int]bool{}
		for _, m := range getPilots(ver) {
			at := [2]int{int(m.x), int(m.y)}
			if known[at] || m.dark != b.modules[m.y][m.x] {
				t.Fatalf("version %d, module %v: repeated or wrong value", ver, at)
			}
			known[at] = true
		}
		for y := 0; y < b.size; y++ {
			for x := 0; x < b.size; x++ {
				if want := b.isFunction[y][x] && !isFormatModule(x, y, b.size); known[[2]int{x, y}] != want {
					t.Fatalf("version %d, module (%d, %d): known %v, want %v", ver, x, y, !want, want)
				}
			}
		}
	}
}

// TestCodewordReliabilityFollowsPlacement checks the decoder's walk of the
// codeword placement against the encoder's map of the bit each module
// carries.
func TestCodewordReliabilityFollowsPlacement(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for ver := MinVersion; ver <= MaxVersion; ver++ {
		b := newBuilder(ver, ECCLow)
		b.drawFunctionPatterns()
		n := b.size
		rel := make([]float32, n*n)
		for i := range rel {
			rel[i] = rng.Float32()
		}
		want := make([]float32, numRawDataModules(ver)/8)
		for i := range want {
			want[i] = float32(math.Inf(1))
		}
		for y, row := range bitMap(b) { // not cached, unlike getTemplate's
			for x, bit := range row {
				if bit >= 0 && int(bit)/8 < len(want) {
					want[bit/8] = min(want[bit/8], rel[y*n+x])
				}
			}
		}
		assertEqual(t, want, b.codewordReliability(rel), "version %d", ver)
	}
}

// TestSoftDecodingBuildsNoTemplates expects the attempts on grids of any
// version to leave the encoder's templates, which hold far more per version
// than decoding needs, unbuilt.
func TestSoftDecodingBuildsNoTemplates(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	tried := 0
	for _, ver := range []int{27, 33, 39} {
		if _, ok := templateCache.Load(ver); ok {
			continue // built by an encoding test
		}
		tried++
		dim := 4*ver + 17
		b := newBuilder(ver, ECCMedium)
		b.drawFunctionPatterns()
		b.drawFormatBits(rng.Intn(8))
		vals := make([]float64, dim*dim)
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				dark := b.modules[y][x]
				if !b.isFunction[y][x] {
					dark = rng.Intn(2) == 0
				}
				v := 200.0
				if dark {
					v = 60
				}
				vals[y*dim+x] = v + rng.NormFloat64()*25
			}
		}
		assertTrue(t, pilotAccuracy(thresholdModules(vals, dim, false)) >= softGate, "version %d grid fails the gate", ver)
		if _, err := readSoft(vals, dim, false, decodeGrid); err == nil {
			t.Fatalf("random version %d grid decoded", ver)
		}
		if _, ok := templateCache.Load(ver); ok {
			t.Errorf("soft decoding built the version %d template", ver)
		}
	}
	if tried == 0 {
		t.Skip("every version tried has a template from another test")
	}
}
