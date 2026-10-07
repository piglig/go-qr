package qr

import (
	"fmt"
	"math"
	"sort"
)

// Robust localization path (M2): handles rotated / noisy images that the
// axis-aligned fast path cannot. It detects the three finder patterns via the
// classic 1:1:3:1:1 run-ratio scan with a vertical cross-check, orders them,
// estimates the symbol dimension, and builds an affine transform (rotation +
// scale + translation) to sample the module grid.
//
// Perspective correction via the alignment pattern is intentionally out of
// scope here — the degraded corpus is rotation + noise, which affine handles
// exactly; camera-perspective skew is a later refinement.

type finderPattern struct {
	x, y       float64 // center in image space
	moduleSize float64
	count      int // number of merged horizontal hits (confidence)
}

// robustSample locates the finder patterns in a binarized image and samples
// the module grid through the affine transform they define.
func robustSample(bm []bool, w, h int) ([][]bool, error) {
	dark := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		return bm[y*w+x]
	}

	finders, err := findFinders(dark, w, h)
	if err != nil {
		return nil, err
	}

	tl, tr, bl := orderFinders(finders)
	moduleSize := (tl.moduleSize + tr.moduleSize + bl.moduleSize) / 3
	if moduleSize <= 0 {
		return nil, fmt.Errorf("%w: bad module size", ErrNotFound)
	}

	dimension, err := computeDimension(tl, tr, bl, moduleSize)
	if err != nil {
		return nil, err
	}
	// From version 7 on, the version information next to the top-right and
	// bottom-left finders gives the exact size; the estimate from finder
	// spacing can be off by a version under rotation or blur.
	if (dimension-17)/4 >= 7 {
		if v, ok := readVersionNear(dark, tl, tr, bl); ok {
			dimension = 4*v + 17
		}
	}

	// Affine: finder centers sit at module (3.5,3.5), (dim-3.5,3.5),
	// (3.5,dim-3.5). Module pitch vectors between TL and the others:
	span := float64(dimension - 7)
	colX := (tr.x - tl.x) / span
	colY := (tr.y - tl.y) / span
	rowX := (bl.x - tl.x) / span
	rowY := (bl.y - tl.y) / span

	modules := make([][]bool, dimension)
	grid := make([]bool, dimension*dimension)
	for r := 0; r < dimension; r++ {
		modules[r] = grid[r*dimension : (r+1)*dimension]
		for c := 0; c < dimension; c++ {
			// module center offset from the TL finder center, in modules
			oc := float64(c) + 0.5 - 3.5
			or := float64(r) + 0.5 - 3.5
			px := tl.x + oc*colX + or*rowX
			py := tl.y + oc*colY + or*rowY
			modules[r][c] = dark(int(px+0.5), int(py+0.5))
		}
	}
	return modules, nil
}

// findFinders scans for finder patterns and returns the three strongest.
func findFinders(dark func(x, y int) bool, w, h int) ([]finderPattern, error) {
	var cands []finderPattern

	add := func(cx, cy, module float64) {
		for i := range cands {
			if math.Abs(cands[i].x-cx) < module && math.Abs(cands[i].y-cy) < module {
				n := float64(cands[i].count)
				cands[i].x = (cands[i].x*n + cx) / (n + 1)
				cands[i].y = (cands[i].y*n + cy) / (n + 1)
				cands[i].moduleSize = (cands[i].moduleSize*n + module) / (n + 1)
				cands[i].count++
				return
			}
		}
		cands = append(cands, finderPattern{x: cx, y: cy, moduleSize: module, count: 1})
	}

	var s [5]int
	for y := 0; y < h; y++ {
		s = [5]int{}
		state := 0
		for x := 0; x < w; x++ {
			if dark(x, y) {
				if state&1 == 1 {
					state++
				}
				s[state]++
			} else {
				if state&1 == 0 {
					if state == 4 {
						if module, ok := checkFinderRatio(s); ok {
							cx := float64(x) - float64(s[4]) - float64(s[3]) - float64(s[2])/2
							total := s[0] + s[1] + s[2] + s[3] + s[4]
							// Confirm vertically, then horizontally through the
							// refined center.
							x0 := int(cx + 0.5)
							if dy, mv, ok := crossCheck(dark, x0, y, 0, 1, s[2], total); ok {
								cy := float64(y) + dy
								if dx, mh, ok := crossCheck(dark, x0, int(cy+0.5), 1, 0, s[2], total); ok {
									add(float64(x0)+dx, cy, (module+mv+mh)/3)
								}
							}
						}
						s[0], s[1], s[2], s[3], s[4] = s[2], s[3], s[4], 1, 0
						state = 3
					} else {
						state++
						s[state]++
					}
				} else {
					s[state]++
				}
			}
		}
	}

	return selectFinders(cands)
}

// selectFinders picks the three candidates that look most like the finders
// of one symbol: similar module sizes, placed at the corners of a right
// isosceles triangle at least 14 modules on a side. Data modules can mimic
// the 1:1:3:1:1 pattern, and the strongest candidates alone are not reliable
// in large symbols.
func selectFinders(cands []finderPattern) ([]finderPattern, error) {
	if len(cands) < 3 {
		return nil, fmt.Errorf("%w: found %d finder patterns", ErrNotFound, len(cands))
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].count > cands[j].count })
	if len(cands) > maxFinderCandidates {
		cands = cands[:maxFinderCandidates]
	}

	best, bestScore := [3]int{}, math.Inf(1)
	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			for k := j + 1; k < len(cands); k++ {
				if s := finderTripleScore(cands[i], cands[j], cands[k]); s < bestScore {
					best, bestScore = [3]int{i, j, k}, s
				}
			}
		}
	}
	if math.IsInf(bestScore, 1) {
		return nil, fmt.Errorf("%w: no three finder patterns form a symbol", ErrNotFound)
	}
	return []finderPattern{cands[best[0]], cands[best[1]], cands[best[2]]}, nil
}

// maxFinderCandidates bounds the O(n³) triple search.
const maxFinderCandidates = 16

// finderTripleScore rates how well three candidates fit one symbol; lower is
// better and +Inf rejects the triple.
func finderTripleScore(a, b, c finderPattern) float64 {
	lo := min(a.moduleSize, b.moduleSize, c.moduleSize)
	hi := max(a.moduleSize, b.moduleSize, c.moduleSize)
	if hi > 1.5*lo {
		return math.Inf(1)
	}
	d := []float64{dist(a, b), dist(b, c), dist(a, c)}
	sort.Float64s(d)
	leg1, leg2, hyp := d[0], d[1], d[2]
	module := (a.moduleSize + b.moduleSize + c.moduleSize) / 3
	if legModules := leg1 / module; legModules < 14*0.8 || legModules > (4*MaxVersion+10)*1.2 {
		return math.Inf(1)
	}
	isosceles := (leg2 - leg1) / leg2
	right := math.Abs(hyp-math.Hypot(leg1, leg2)) / hyp
	if isosceles > 0.2 || right > 0.1 {
		return math.Inf(1)
	}
	// Prefer candidates confirmed on many rows.
	support := 1 / float64(min(a.count, b.count, c.count))
	return isosceles + right + (hi-lo)/hi + 0.1*support
}

// checkFinderRatio reports whether the five run lengths match 1:1:3:1:1 and
// returns the estimated module size.
func checkFinderRatio(s [5]int) (float64, bool) {
	total := 0
	for _, v := range s {
		if v == 0 {
			return 0, false
		}
		total += v
	}
	if total < 7 {
		return 0, false
	}
	module := float64(total) / 7
	maxVar := module / 2
	if math.Abs(module-float64(s[0])) < maxVar &&
		math.Abs(module-float64(s[1])) < maxVar &&
		math.Abs(3*module-float64(s[2])) < 3*maxVar &&
		math.Abs(module-float64(s[3])) < maxVar &&
		math.Abs(module-float64(s[4])) < maxVar {
		return module, true
	}
	return 0, false
}

// crossCheck confirms a candidate by measuring the 1:1:3:1:1 runs through
// (cx, cy) along direction (dx, dy). It returns the refined center
// coordinate along that axis and the module size of the runs. The total run
// length must be within 40% of originalTotal, the length seen by the scan that
// produced the candidate.
func crossCheck(dark func(x, y int) bool, cx, cy, dx, dy, maxCount, originalTotal int) (float64, float64, bool) {
	// dark reports false outside the image, so every loop ends: dark runs
	// stop at the border and light runs are bounded by maxCount.
	var s [5]int
	at := func(i int) bool { return dark(cx+i*dx, cy+i*dy) }

	i := 0
	for at(i) && s[2] <= 4*maxCount {
		s[2]++
		i--
	}
	for ; !at(i) && s[1] <= maxCount; i-- {
		s[1]++
	}
	for ; at(i) && s[0] <= maxCount; i-- {
		s[0]++
	}
	if s[0] == 0 || s[1] > maxCount || s[0] > maxCount {
		return 0, 0, false
	}

	i = 1
	for at(i) && s[2] <= 4*maxCount {
		s[2]++
		i++
	}
	for ; !at(i) && s[3] <= maxCount; i++ {
		s[3]++
	}
	for ; at(i) && s[4] <= maxCount; i++ {
		s[4]++
	}
	if s[4] == 0 || s[3] > maxCount || s[4] > maxCount {
		return 0, 0, false
	}

	total := s[0] + s[1] + s[2] + s[3] + s[4]
	if 5*abs(total-originalTotal) >= 2*originalTotal {
		return 0, 0, false
	}
	module, ok := checkFinderRatio(s)
	if !ok {
		return 0, 0, false
	}
	// Center relative to the start point, along the axis.
	center := float64(i) - float64(s[4]) - float64(s[3]) - float64(s[2])/2
	return center, module, true
}

// orderFinders identifies which of the three patterns is top-left, top-right,
// and bottom-left. The top-left is the vertex of the right angle (opposite the
// longest edge); handedness picks TR vs BL.
func orderFinders(p []finderPattern) (tl, tr, bl finderPattern) {
	d01 := dist(p[0], p[1])
	d12 := dist(p[1], p[2])
	d02 := dist(p[0], p[2])

	// vertex = point not on the longest (hypotenuse) edge
	var a, c finderPattern
	switch {
	case d12 >= d01 && d12 >= d02:
		tl, a, c = p[0], p[1], p[2]
	case d02 >= d01 && d02 >= d12:
		tl, a, c = p[1], p[0], p[2]
	default:
		tl, a, c = p[2], p[0], p[1]
	}

	// cross product (a-tl) x (c-tl); in image (y-down) coords a positive cross
	// means a is to the right (top-right) and c is below (bottom-left).
	cross := (a.x-tl.x)*(c.y-tl.y) - (a.y-tl.y)*(c.x-tl.x)
	if cross >= 0 {
		tr, bl = a, c
	} else {
		tr, bl = c, a
	}
	return tl, tr, bl
}

func dist(a, b finderPattern) float64 {
	dx := a.x - b.x
	dy := a.y - b.y
	return math.Sqrt(dx*dx + dy*dy)
}

// computeDimension estimates the module count per side from finder spacing and
// snaps it to the nearest valid QR dimension (size = 4*version + 17).
//
// Rounding the version directly is more robust than the classic mod-4 bit
// twiddle: under rotation/noise the raw estimate can land two off a valid
// dimension (estimate % 4 == 3), which the old logic rejected outright. Here
// any estimate snaps to its nearest version and only fails when the rounded
// version falls outside the supported range.
func computeDimension(tl, tr, bl finderPattern, moduleSize float64) (int, error) {
	tlbr := dist(tl, tr) / moduleSize
	tlbl := dist(tl, bl) / moduleSize
	raw := (tlbr+tlbl)/2 + 7 // finder centers span size-7 modules

	ver := int(math.Round((raw - 17) / 4))
	if ver < MinVersion || ver > MaxVersion {
		return 0, fmt.Errorf("%w: estimated dimension %.1f maps to version %d", ErrNotFound, raw, ver)
	}
	return ver*4 + 17, nil
}

// readVersionNear reads the two 18-bit version information blocks by
// sampling relative to the finder beside each one, using that finder's own
// module size, so the result does not depend on the estimated dimension. It
// returns the first block that BCH-corrects to a valid version.
func readVersionNear(dark func(x, y int) bool, tl, tr, bl finderPattern) (int, bool) {
	sample := func(f finderPattern, ux, uy, vx, vy float64, du, dv func(i int) float64) int {
		bits := 0
		for i := 0; i < 18; i++ {
			px := f.x + du(i)*ux + dv(i)*vx
			py := f.y + du(i)*uy + dv(i)*vy
			if dark(int(px+0.5), int(py+0.5)) {
				bits |= 1 << i
			}
		}
		return bits
	}
	// unit returns the vector from a to b scaled to one module of size m.
	unit := func(a, b finderPattern, m float64) (float64, float64) {
		d := dist(a, b)
		return (b.x - a.x) / d * m, (b.y - a.y) / d * m
	}

	// Block 1 sits left of the top-right finder: bit i is module
	// (size-11+i%3, i/3), offset (i%3-7, i/3-3) from the finder center.
	cx, cy := unit(tl, tr, tr.moduleSize)
	rx, ry := unit(tl, bl, tr.moduleSize)
	if v, ok := correctVersion(sample(tr, cx, cy, rx, ry,
		func(i int) float64 { return float64(i%3 - 7) },
		func(i int) float64 { return float64(i/3 - 3) })); ok {
		return v, true
	}

	// Block 2 is its transpose, above the bottom-left finder.
	cx, cy = unit(tl, tr, bl.moduleSize)
	rx, ry = unit(tl, bl, bl.moduleSize)
	return correctVersion(sample(bl, cx, cy, rx, ry,
		func(i int) float64 { return float64(i/3 - 3) },
		func(i int) float64 { return float64(i%3 - 7) }))
}

// versionBits returns the 18-bit version information codeword: the version
// followed by its BCH(18,6) remainder (ISO/IEC 18004 Annex D).
func versionBits(ver int) int {
	rem := ver
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	return ver<<12 | rem
}

// correctVersion returns the version whose codeword is nearest to bits, if it
// is within the 3-bit correction capacity of the code.
func correctVersion(bits int) (int, bool) {
	best, bestDist := 0, 4
	for v := 7; v <= MaxVersion; v++ {
		if d := bitCount(versionBits(v) ^ bits); d < bestDist {
			best, bestDist = v, d
		}
	}
	return best, bestDist <= 3
}
