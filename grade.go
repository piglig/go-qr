package qr

import (
	"fmt"
	"image"
	"math"

	"github.com/piglig/go-qr/v2/internal/symbol"
)

// QualityGrade is a symbol quality grade of ISO/IEC 15415, from GradeA, the
// best, to GradeF, a fail. A higher grade is better. Since v2.7.
type QualityGrade int8

// The grades of ISO/IEC 15415, which also writes them 4 to 0.
const (
	GradeF QualityGrade = iota
	GradeD
	GradeC
	GradeB
	GradeA
)

// String returns the grade's letter, "A" to "F".
func (g QualityGrade) String() string {
	if g < GradeF || g > GradeA {
		return fmt.Sprintf("QualityGrade(%d)", int8(g))
	}
	return string("FDCBA"[g])
}

// QualityParameter is one measured parameter of a Quality and its grade.
// Since v2.7.
type QualityParameter struct {
	Value float64
	Grade QualityGrade
}

// Quality is the print quality of a symbol in an image, by the parameters
// of ISO/IEC 15415 for two-dimensional matrix symbols, as Grade measures
// them. Since v2.7.
type Quality struct {
	// Grade is the lowest grade of the parameters below.
	Grade QualityGrade

	// SymbolContrast is the difference between the highest and the lowest
	// reflectance in the symbol and its quiet zone, from 0 to 1.
	// A ≥ 0.70, B ≥ 0.55, C ≥ 0.40, D ≥ 0.20.
	SymbolContrast QualityParameter
	// Modulation grades how evenly dark and light the modules are. Each
	// module is graded by its distance from the global threshold, midway
	// between the highest and the lowest reflectance, relative to half the
	// symbol contrast (A ≥ 0.50, B ≥ 0.40, C ≥ 0.30, D ≥ 0.20), and each
	// codeword by its worst module. At each grade level, the codewords
	// below it are taken as erasures, and the level counts for no more
	// than the grade of the unused error correction they leave; the best
	// level is the grade. Value is the lowest module's.
	Modulation QualityParameter
	// ReflectanceMargin is Modulation for modules on the side of the
	// threshold their color belongs on; a module on the wrong side grades
	// F. Value is the lowest module's, negative for one on the wrong side.
	ReflectanceMargin QualityParameter
	// FixedPatternDamage grades the three finder patterns with their
	// separators. At each grade level, each finder is graded by its
	// modules whose reflectance margin is below the level, A for none,
	// B for one, C for two, D for three and F for more; the level counts
	// for no more than the worst finder's grade, and the best level is the
	// grade. Value is the share of their modules on the wrong side of the
	// threshold.
	FixedPatternDamage QualityParameter
	// AxialNonuniformity is the difference between the mean module pitch
	// along the symbol's two axes, relative to their mean.
	// A ≤ 0.06, B ≤ 0.08, C ≤ 0.10, D ≤ 0.12.
	AxialNonuniformity QualityParameter
	// GridNonuniformity is the largest distance, in modules, of an
	// alignment pattern's center from where the regular grid through the
	// finder patterns' centers puts it; 0 for version 1, which has none.
	// A ≤ 0.38, B ≤ 0.50, C ≤ 0.63, D ≤ 0.75.
	GridNonuniformity QualityParameter
	// UnusedErrorCorrection is that of the decode, as in DecodeResult.
	// A ≥ 0.62, B ≥ 0.50, C ≥ 0.37, D ≥ 0.25.
	UnusedErrorCorrection QualityParameter

	// PixelsPerModule is the mean module pitch in the image.
	PixelsPerModule float64
	// Result is the decoded symbol.
	Result *DecodeResult
}

// Grade decodes the QR Code in img, as Decode does but by hard decisions
// only, and grades its print quality by the parameters of ISO/IEC 15415.
// Use it to compare printed codes, or renderings with colors and a logo,
// and to watch codes wear. Applications commonly ask for grade C or
// better.
//
// The grades are estimates, not a verifier's. Reflectance is estimated
// from the image's luminance, linearized as sRGB, relative to white;
// modules are measured at the centers the decoder located, over a disc
// of 0.6 modules; and one image counts, where a verifier calibrates its
// light and averages several. Axial and grid nonuniformity assume the
// image looks squarely at the symbol, as a verifier's camera does:
// perspective lowers them. Modules should span 5 pixels or more.
//
// The error is that of Decode, or wraps ErrDecodeFailed when only soft
// decisions read the symbol. Since v2.7.
func Grade(img image.Image, opts ...DecodeOption) (*Quality, error) {
	var cfg decodeConfig
	for _, o := range opts {
		o(&cfg)
	}
	res, err := searchImage(img, cfg, func(grid [][]bool, rel []float32) (*DecodeResult, error) {
		if rel != nil {
			return nil, fmt.Errorf("%w: only soft decisions read the symbol", ErrDecodeFailed)
		}
		res, err := decodeGrid(grid, nil)
		if res != nil {
			res.centers = &moduleCenters{}
		}
		return res, err
	})
	if err != nil {
		return nil, err
	}
	want, err := newCode(res.Version, res.ECC, res.codewords, res.Mask)
	if err != nil {
		return nil, fmt.Errorf("%w: re-encode the symbol: %v", ErrDecodeFailed, err)
	}
	b := img.Bounds()
	q := grade(toLuma(img), b.Dx(), b.Dy(), res, want)
	for i := range res.Corners {
		res.Corners[i] = res.Corners[i].Add(b.Min)
	}
	res.centers = nil
	return q, nil
}

// moduleCenters holds the image positions of the centers of a sampled
// grid's modules and of a margin of modules around it, row by row. Grade
// asks locate for them by setting DecodeResult.centers.
type moduleCenters struct {
	dim  int
	x, y []float64
}

// centersMargin is the margin of modules around the symbol whose centers
// are kept: the quiet zone that the symbol contrast takes in, and one more
// for the module axes there.
const centersMargin = 3

// fill computes the centers of a dim×dim grid that at maps from module
// space.
func (c *moduleCenters) fill(at func(u, v float64) (float64, float64), dim int) {
	n := dim + 2*centersMargin
	c.dim = dim
	c.x, c.y = make([]float64, n*n), make([]float64, n*n)
	for r := 0; r < n; r++ {
		for col := 0; col < n; col++ {
			c.x[r*n+col], c.y[r*n+col] = at(float64(col-centersMargin)+0.5, float64(r-centersMargin)+0.5)
		}
	}
}

// shift moves the centers by d pixels.
func (c *moduleCenters) shift(d image.Point) {
	for i := range c.x {
		c.x[i] += float64(d.X)
		c.y[i] += float64(d.Y)
	}
}

// symbolFrame gives the centers of a symbol's modules in its own
// orientation: a mirrored symbol was read transposed.
type symbolFrame struct {
	c        *moduleCenters
	mirrored bool
}

// at returns the center of module (x, y) of the symbol, which may lie up
// to centersMargin modules outside it.
func (f symbolFrame) at(x, y int) (float64, float64) {
	if f.mirrored {
		x, y = y, x
	}
	n := f.c.dim + 2*centersMargin
	i := (y+centersMargin)*n + x + centersMargin
	return f.c.x[i], f.c.y[i]
}

// axes returns the image vectors of one module along the symbol's x and y
// axes at module (x, y), from its neighbors' centers.
func (f symbolFrame) axes(x, y int) (ux, uy, vx, vy float64) {
	lo, hi := -centersMargin, f.c.dim+centersMargin-1
	x0, x1 := max(x-1, lo), min(x+1, hi)
	y0, y1 := max(y-1, lo), min(y+1, hi)
	ax, ay := f.at(x0, y)
	bx, by := f.at(x1, y)
	cx, cy := f.at(x, y0)
	dx, dy := f.at(x, y1)
	return (bx - ax) / float64(x1-x0), (by - ay) / float64(x1-x0), (dx - cx) / float64(y1-y0), (dy - cy) / float64(y1-y0)
}

// apertureRadius is the radius, in modules, of the disc over which a
// module's reflectance is measured, within the 0.8 modules that
// ISO/IEC 15415 commonly sets the aperture to.
const apertureRadius = 0.3

// srgbLinear maps 8-bit sRGB values to linear light, from 0 to 1.
var srgbLinear = func() (t [256]float64) {
	for i := range t {
		v := float64(i) / 255
		if v <= 0.04045 {
			t[i] = v / 12.92
		} else {
			t[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return t
}()

// grade measures the quality of symbol res, located in the w×h luminance
// image l, against want, the symbol re-encoded from its data.
func grade(l []uint8, w, h int, res *DecodeResult, want *Code) *Quality {
	f := symbolFrame{res.centers, res.Mirrored}
	dim := want.size
	q := &Quality{Result: res}

	// The reflectance of each module and of the quiet zone, over a disc:
	// its center and eight points around it.
	const qz = centersMargin - 1
	n := dim + 2*qz
	refl := make([]float64, n*n)
	var disc [9][2]float64
	for i := range disc {
		ox, oy := float64(i%3-1), float64(i/3-1)
		s := apertureRadius / math.Max(1, math.Hypot(ox, oy))
		disc[i] = [2]float64{ox * s, oy * s}
	}
	rmin, rmax := math.Inf(1), math.Inf(-1)
	pitchX, pitchY := 0.0, 0.0
	for y := -qz; y < dim+qz; y++ {
		for x := -qz; x < dim+qz; x++ {
			inside := x >= 0 && y >= 0 && x < dim && y < dim
			cx, cy := f.at(x, y)
			ux, uy, vx, vy := f.axes(x, y)
			if inside {
				pitchX += math.Hypot(ux, uy)
				pitchY += math.Hypot(vx, vy)
			}
			sum, k := 0.0, 0
			for _, o := range disc {
				px, py := cx+o[0]*ux+o[1]*vx, cy+o[0]*uy+o[1]*vy
				if px >= 0 && py >= 0 && px <= float64(w-1) && py <= float64(h-1) {
					sum += srgbLinear[uint8(math.Round(lumaAt(l, w, h, px, py)))]
					k++
				}
			}
			if k == 0 {
				continue // a quiet zone module outside the image
			}
			r := sum / float64(k)
			refl[(y+qz)*n+x+qz] = r
			rmin, rmax = min(rmin, r), max(rmax, r)
		}
	}
	sc := rmax - rmin
	q.SymbolContrast = QualityParameter{sc, gradeAtLeast(sc, 0.70, 0.55, 0.40, 0.20)}
	gt := (rmax + rmin) / 2

	// The modulation and reflectance margin of each module of the symbol,
	// by the color it was encoded with.
	mod := make([]float64, dim*dim)
	margin := make([]float64, dim*dim)
	minMod, minMargin := math.Inf(1), math.Inf(1)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			d := 0.0
			if sc > 0 {
				d = 2 * (refl[(y+qz)*n+x+qz] - gt) / sc // positive when light
			}
			if want.modules[y][x] != res.Inverted {
				d = -d // a module meant to look dark
			}
			mod[y*dim+x], margin[y*dim+x] = math.Abs(d), d
			minMod, minMargin = min(minMod, math.Abs(d)), min(minMargin, d)
		}
	}
	cw := newCodewordMap(res.Version, res.ECC)
	q.Modulation = QualityParameter{minMod, cw.grade(mod)}
	q.ReflectanceMargin = QualityParameter{minMargin, cw.grade(margin)}
	q.FixedPatternDamage = finderDamage(margin, dim)

	pitchX /= float64(dim * dim)
	pitchY /= float64(dim * dim)
	q.PixelsPerModule = (pitchX + pitchY) / 2
	an := math.Abs(pitchX-pitchY) / q.PixelsPerModule
	q.AxialNonuniformity = QualityParameter{an, gradeAtMost(an, 0.06, 0.08, 0.10, 0.12)}
	gn := gridNonuniformity(f, res.Version, q.PixelsPerModule)
	q.GridNonuniformity = QualityParameter{gn, gradeAtMost(gn, 0.38, 0.50, 0.63, 0.75)}
	q.UnusedErrorCorrection = QualityParameter{res.UnusedErrorCorrection, uecGrade(res.UnusedErrorCorrection)}

	q.Grade = min(q.SymbolContrast.Grade, q.Modulation.Grade, q.ReflectanceMargin.Grade,
		q.FixedPatternDamage.Grade, q.AxialNonuniformity.Grade, q.GridNonuniformity.Grade,
		q.UnusedErrorCorrection.Grade)
	return q
}

// gradeAtLeast grades v by the lowest values of grades A, B, C and D.
func gradeAtLeast(v, a, b, c, d float64) QualityGrade {
	switch {
	case v >= a:
		return GradeA
	case v >= b:
		return GradeB
	case v >= c:
		return GradeC
	case v >= d:
		return GradeD
	}
	return GradeF
}

// gradeAtMost grades v by the highest values of grades A, B, C and D.
func gradeAtMost(v, a, b, c, d float64) QualityGrade {
	return gradeAtLeast(-v, -a, -b, -c, -d)
}

// moduleGrade grades a module's modulation or reflectance margin.
func moduleGrade(v float64) QualityGrade { return gradeAtLeast(v, 0.50, 0.40, 0.30, 0.20) }

// uecGrade grades unused error correction.
func uecGrade(v float64) QualityGrade { return gradeAtLeast(v, 0.62, 0.50, 0.37, 0.25) }

// codewordMap gives the codeword of each module of a symbol of a version
// and level, and the block of each codeword.
type codewordMap struct {
	bitAt    [][]int16 // data bit of each module, in placement order; -1 for none
	block    []int     // block of each codeword, in placement order
	blocks   int
	capacity int // check codewords per block less misdecode protection
}

func newCodewordMap(ver int, ecl ECC) codewordMap {
	l := symbol.Level(ecl)
	return codewordMap{
		bitAt:    symbol.BitAt(ver),
		block:    symbol.CodewordBlocks(ver, l),
		blocks:   int(symbol.NumErrorCorrectionBlocks[l][ver]),
		capacity: int(symbol.ECCCodewordsPerBlock[l][ver]) - symbol.MisdecodeProtection(ver, l),
	}
}

// grade grades a symbol by the modulation or reflectance margin of each
// of its modules, row by row: each codeword by its worst module, and the
// symbol, at each grade level, by the lower of the level and the grade of
// the unused error correction were the codewords below the level erased;
// the best level counts.
func (m codewordMap) grade(vals []float64) QualityGrade {
	cw := make([]QualityGrade, len(m.block))
	for i := range cw {
		cw[i] = GradeA
	}
	dim := len(m.bitAt)
	for y, row := range m.bitAt {
		for x, bit := range row {
			if bit >= 0 {
				cw[bit/8] = min(cw[bit/8], moduleGrade(vals[y*dim+x]))
			}
		}
	}
	best := GradeF
	erased := make([]int, m.blocks)
	for level := GradeA; level > GradeF; level-- {
		clear(erased)
		for c, g := range cw {
			if g < level {
				erased[m.block[c]]++
			}
		}
		uec := 1.0
		for _, e := range erased {
			uec = min(uec, max(0, 1-float64(e)/float64(m.capacity)))
		}
		best = max(best, min(level, uecGrade(uec)))
	}
	return best
}

// finderDamage grades the three finder patterns of a dim×dim symbol, each
// with its separator, by the reflectance margins of the symbol's modules,
// row by row; see Quality.FixedPatternDamage.
func finderDamage(margin []float64, dim int) QualityParameter {
	origins := [3][2]int{{0, 0}, {dim - 8, 0}, {0, dim - 8}}
	wrong, total := 0, 0
	for _, o := range origins {
		for y := o[1]; y < o[1]+8; y++ {
			for x := o[0]; x < o[0]+8; x++ {
				total++
				if margin[y*dim+x] < 0 {
					wrong++
				}
			}
		}
	}
	best := GradeF
	for level := GradeA; level > GradeF; level-- {
		worst := GradeA
		for _, o := range origins {
			below := 0
			for y := o[1]; y < o[1]+8; y++ {
				for x := o[0]; x < o[0]+8; x++ {
					if moduleGrade(margin[y*dim+x]) < level {
						below++
					}
				}
			}
			worst = min(worst, GradeA-QualityGrade(min(below, int(GradeA))))
		}
		best = max(best, min(level, worst))
	}
	return QualityParameter{float64(wrong) / float64(total), best}
}

// gridNonuniformity returns the largest distance, in modules of pitch
// pixels, of an alignment pattern's center from where the regular grid
// through the finder patterns' centers puts it.
func gridNonuniformity(f symbolFrame, ver int, pitch float64) float64 {
	pos := symbol.AlignmentPositions(ver)
	if len(pos) == 0 || pitch <= 0 {
		return 0
	}
	dim := 4*ver + 17
	ox, oy := f.at(3, 3)
	rx, ry := f.at(dim-4, 3)
	bx, by := f.at(3, dim-4)
	span := float64(dim - 7)
	worst := 0.0
	for i, py := range pos {
		for j, px := range pos {
			if i == 0 && j == 0 || i == 0 && j == len(pos)-1 || i == len(pos)-1 && j == 0 {
				continue // a finder's place
			}
			s, t := float64(px-3)/span, float64(py-3)/span
			ex, ey := ox+s*(rx-ox)+t*(bx-ox), oy+s*(ry-oy)+t*(by-oy)
			ax, ay := f.at(px, py)
			worst = max(worst, math.Hypot(ax-ex, ay-ey)/pitch)
		}
	}
	return worst
}
