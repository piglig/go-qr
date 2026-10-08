package qr

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Robust localization path: handles rotated, tilted and noisy images that the
// axis-aligned fast path cannot. It detects finder patterns with the classic
// 1:1:3:1:1 run-ratio scan and cross-checks, and picks the triples that look
// like one symbol. The module grid is then located by the structural fit in
// decode_fit.go, which measures the finders' nested squares and grows a
// homography over the alignment patterns, and is verified by the timing
// patterns before anything is decoded.
//
// When the finders' edges cannot be measured, as with round finder styles,
// the grid is sampled through a perspective transform whose fourth point is
// estimated instead, at most two ways, most precise first:
//
//  1. the bottom-right alignment pattern (version 2 and up), searched for
//     around the position the next estimate predicts;
//  2. the intersection of the outer edges of the top-right and bottom-left
//     finders, or, when those edges cannot be traced, the parallelogram
//     completion of the three finder centers, which is exact without
//     perspective.
//
// This fallback is the scheme of ZXing and zxing-cpp.

type finderPattern struct {
	x, y       float64 // center in image space
	moduleSize float64 // from horizontal and vertical runs
	count      int     // number of merged horizontal hits (confidence)
}

// robustDecode locates symbols in a binarized image and passes candidate
// module grids to read until one decodes. It returns the first error
// otherwise.
//
// tm thresholds the image at 1/scale of its resolution, inverted for light
// symbols on dark; the symbol is located there, and its modules are read
// from the full-resolution luminance l (lw×lh pixels).
func robustDecode(tm *thresholdMap, inverted bool, scale int, l []uint8, lw, lh int, read func([][]bool) (*DecodeResult, error)) (*DecodeResult, error) {
	readGrid := func(p mapper, dim int) (*DecodeResult, error) {
		if scale > 1 {
			p = scaledMapper{p, float64(scale)}
		}
		return read(readModules(l, lw, lh, p, dim, inverted))
	}
	dark := func(x, y int) bool { return tm.dark(x, y) != inverted && x >= 0 && y >= 0 && x < tm.w && y < tm.h }
	w, h := tm.w, tm.h
	step := 1
	if scale == 1 {
		step = finderRowStep(w * h)
	}
	triples, err := findFindersStep(tm, inverted, dark, step)
	if err != nil {
		return nil, err
	}

	// Fit the best triple with the usual corner assignment. If its timing
	// patterns do not read back well, the triple may be a false one or its
	// top-left finder misjudged, which strong perspective causes: the other
	// assignments and triples are fitted too, and one of them replaces the
	// first only if its timing patterns read back well. Fitting costs a
	// small fraction of a decode, and only one grid is decoded.
	var (
		model    *gridModel
		dim      int
		score    = -1.0
		first    *symbolGeometry
		firstErr error
	)
search:
	for ti, t := range triples {
		for k := 0; k < 3; k++ {
			if ti > 0 && k > 0 {
				break // other assignments only for the strongest triple
			}
			g, err := newSymbolGeometryCorner(dark, t, k)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if first == nil {
				first = g
			}
			m, d, s, ok := g.fitSymbol(dark)
			if ok && (g == first || s >= goodTimingScore && s > score) {
				model, dim, score = m, d, s
			}
			if score >= goodTimingScore {
				break search
			}
		}
	}
	if model != nil {
		return readGrid(model, dim)
	}
	if first == nil {
		return nil, firstErr
	}
	// The finders' corners could not be measured, as with circular finder
	// styles: estimate the fourth point instead.
	g := first
	for _, p := range g.transforms(dark) {
		res, err := readGrid(p, g.dim)
		if err == nil || errors.Is(err, ErrUnsupported) {
			return res, err
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

// symbolGeometry is a symbol located by its three finder patterns.
type symbolGeometry struct {
	tl, tr, bl finderPattern
	// Unit vectors along the top (TL→TR) and left (TL→BL) edges.
	cx, cy, rx, ry float64
	// Module sizes of each finder along the top and left edge directions.
	// Runs measured along the image axes overestimate the module size by up
	// to √2 in a rotated symbol, so they are not used for geometry.
	tlC, tlR, trC, trR, blC, blR float64
	dim                          int
}

// goodTimingScore is the timing pattern agreement at which a fit is accepted
// without trying other finder assignments.
const goodTimingScore = 0.9

func newSymbolGeometry(dark func(x, y int) bool, t [3]finderPattern) (*symbolGeometry, error) {
	return newSymbolGeometryCorner(dark, t, 0)
}

// newSymbolGeometryCorner locates a symbol from a finder triple. k selects
// the top-left finder: 0 is the vertex of the right angle as orderFinders
// judges it, 1 and 2 the other two candidates.
func newSymbolGeometryCorner(dark func(x, y int) bool, t [3]finderPattern, k int) (*symbolGeometry, error) {
	tl, tr, bl := orderFinders(t[:])
	switch k {
	case 1:
		tl, tr, bl = handed(tr, tl, bl)
	case 2:
		tl, tr, bl = handed(bl, tl, tr)
	}
	g := &symbolGeometry{tl: tl, tr: tr, bl: bl}
	top, left := dist(tl, tr), dist(tl, bl)
	if top == 0 || left == 0 {
		return nil, fmt.Errorf("%w: degenerate finder triple", ErrNotFound)
	}
	g.cx, g.cy = (tr.x-tl.x)/top, (tr.y-tl.y)/top
	g.rx, g.ry = (bl.x-tl.x)/left, (bl.y-tl.y)/left
	g.tlC = finderModuleAlong(dark, tl, g.cx, g.cy)
	g.trC = finderModuleAlong(dark, tr, g.cx, g.cy)
	g.blC = finderModuleAlong(dark, bl, g.cx, g.cy)
	g.tlR = finderModuleAlong(dark, tl, g.rx, g.ry)
	g.trR = finderModuleAlong(dark, tr, g.rx, g.ry)
	g.blR = finderModuleAlong(dark, bl, g.rx, g.ry)

	// Finder centers span size-7 modules along each edge. Under perspective
	// the module size varies along the edge; the mean of the two ends is a
	// good estimate of the mean over it.
	raw := (top/((g.tlC+g.trC)/2) + left/((g.tlR+g.blR)/2)) / 2
	ver := int(math.Round((raw + 7 - 17) / 4))
	if ver < MinVersion || ver > MaxVersion {
		return nil, fmt.Errorf("%w: estimated dimension %.1f maps to version %d", ErrNotFound, raw+7, ver)
	}
	// From version 7 on, the version information next to the top-right and
	// bottom-left finders gives the exact size; the estimate from finder
	// spacing can be off by a version under perspective or blur.
	if ver >= 7 {
		if v, ok := g.readVersion(dark); ok {
			ver = v
		}
	}
	g.dim = 4*ver + 17
	return g, nil
}

// finderModuleAlong measures a finder's module size along the unit direction
// (dx, dy) from the extent of its 7-module width through the center. It
// falls back to the axis-run estimate when the edges cannot be traced.
func finderModuleAlong(dark func(x, y int) bool, f finderPattern, dx, dy float64) float64 {
	a, okA := halfFinderLength(dark, f, dx, dy)
	b, okB := halfFinderLength(dark, f, -dx, -dy)
	if !okA || !okB {
		return f.moduleSize
	}
	return (a + b) / 7
}

// halfFinderLength walks from a finder's center along (dx, dy) across the
// dark center, the light ring and the dark ring, and returns the distance to
// the outer edge: 3.5 modules.
func halfFinderLength(dark func(x, y int) bool, f finderPattern, dx, dy float64) (float64, bool) {
	// Step one pixel along the major axis, as a line rasterizer would.
	step := 1 / math.Max(math.Abs(dx), math.Abs(dy))
	ux, uy := dx*step, dy*step
	limit := int(8 * f.moduleSize * math.Sqrt2)
	state := 0 // 0 dark center, 1 light ring, 2 dark ring
	for i := 1; i <= limit; i++ {
		d := dark(int(math.Floor(f.x+ux*float64(i))), int(math.Floor(f.y+uy*float64(i))))
		switch {
		case state == 0 && !d, state == 1 && d:
			state++
		case state == 2 && !d:
			return (float64(i) - 0.5) * step, true
		}
	}
	return 0, false
}

// affine returns the image position of module coordinates (u, v) under the
// affine transform the finder centers define.
func (g *symbolGeometry) affine(u, v float64) (float64, float64) {
	span := float64(g.dim - 7)
	su, sv := (u-3.5)/span, (v-3.5)/span
	return g.tl.x + su*(g.tr.x-g.tl.x) + sv*(g.bl.x-g.tl.x),
		g.tl.y + su*(g.tr.y-g.tl.y) + sv*(g.bl.y-g.tl.y)
}

// transforms returns the candidate module-to-image transforms, most precise
// first: at most two.
func (g *symbolGeometry) transforms(dark func(x, y int) bool) []perspective {
	d := float64(g.dim)
	finders := func(u4, v4, x4, y4 float64) (perspective, bool) {
		return quadToQuad(
			[4][2]float64{{3.5, 3.5}, {d - 3.5, 3.5}, {u4, v4}, {3.5, d - 3.5}},
			[4][2]float64{{g.tl.x, g.tl.y}, {g.tr.x, g.tr.y}, {x4, y4}, {g.bl.x, g.bl.y}},
		)
	}
	var out []perspective

	px, py := g.affine(d-3.5, d-3.5)
	parallelogram, okP := finders(d-3.5, d-3.5, px, py)

	var edge perspective
	okE := false
	if x, y, ok := g.edgeCorner(dark); ok {
		// Reject corners far from the parallelogram estimate: a traced edge
		// that ran into data modules. Perspective alone moves the corner by
		// a fair fraction of the symbol, so the bound is loose; a corner
		// that is wrong but within it fails to decode, it does not misread.
		ax, ay := g.affine(d, d)
		if math.Hypot(x-ax, y-ay) < 0.5*float64(g.dim)*(g.trC+g.blR)/2 {
			edge, okE = finders(d, d, x, y)
		}
	}

	if g.dim > 21 {
		guess := parallelogram
		if okE {
			guess = edge
		}
		if okE || okP {
			if x, y, ok := findAlignment(dark, guess, d-6.5); ok {
				if p, ok := finders(d-6.5, d-6.5, x, y); ok {
					out = append(out, p)
				}
			}
		}
	}
	switch {
	case okE:
		out = append(out, edge)
	case okP:
		out = append(out, parallelogram)
	}
	return out
}

// edgeCorner estimates the outer bottom-right corner of the symbol as the
// intersection of the right edge of the top-right finder and the bottom
// edge of the bottom-left finder.
func (g *symbolGeometry) edgeCorner(dark func(x, y int) bool) (float64, float64, bool) {
	right, ok := traceFinderEdge(dark, g.tr, g.cx, g.cy, g.trC, g.rx, g.ry, g.trR)
	if !ok {
		return 0, 0, false
	}
	bottom, ok := traceFinderEdge(dark, g.bl, g.rx, g.ry, g.blR, g.cx, g.cy, g.blC)
	if !ok {
		return 0, 0, false
	}
	return right.intersect(bottom)
}

// traceFinderEdge fits a line to the outer edge of finder f on its side
// toward (nx, ny): it starts in the middle of the outer dark ring at points
// spread along the edge direction (ex, ey) and walks outward to the first
// light pixel. nm and em are the finder's module sizes along the two
// directions.
func traceFinderEdge(dark func(x, y int) bool, f finderPattern, nx, ny, nm, ex, ey, em float64) (line, bool) {
	var pts [][2]float64
	const step = 0.25
	for s := -2.5; s <= 2.5; s += 0.5 {
		x := f.x + 3*nm*nx + s*em*ex
		y := f.y + 3*nm*ny + s*em*ey
		if !dark(int(math.Floor(x)), int(math.Floor(y))) {
			continue
		}
		for i := 1; float64(i)*step <= 2*nm; i++ {
			qx, qy := x+float64(i)*step*nx, y+float64(i)*step*ny
			if !dark(int(math.Floor(qx)), int(math.Floor(qy))) {
				pts = append(pts, [2]float64{qx - step/2*nx, qy - step/2*ny})
				break
			}
		}
	}
	if len(pts) < 6 {
		return line{}, false
	}
	return fitLine(pts)
}

// findAlignment searches for the alignment pattern centered at module
// (c, c) near where guess maps it, in windows of 4, 8 and 16 modules.
func findAlignment(dark func(x, y int) bool, guess perspective, c float64) (float64, float64, bool) {
	ex, ey := guess.apply(c, c)
	x1, y1 := guess.apply(c+1, c)
	x2, y2 := guess.apply(c, c+1)
	return searchAlignment(dark, ex, ey, x1-ex, y1-ey, x2-ex, y2-ey, []float64{4, 8, 16})
}

// searchAlignment looks for an alignment pattern around (ex, ey), where one
// module spans (ux, uy) and (vx, vy). It scores a template of the pattern's
// center, light ring and dark ring, transformed by that local geometry, at
// positions on rings of growing distance, and refines the match to the
// center of its dark module. A perfect match ends the search; otherwise the
// best match with at most one mismatch is taken once the rings reach one of
// radii (in modules, ascending).
func searchAlignment(dark func(x, y int) bool, ex, ey, ux, uy, vx, vy float64, radii []float64) (float64, float64, bool) {
	at := func(x, y, a, b float64) bool {
		return dark(int(math.Floor(x+a*ux+b*vx)), int(math.Floor(y+a*uy+b*vy)))
	}
	// score counts the template points that match, giving up once two
	// differ: only matches with at most one mismatch are of interest.
	score := func(x, y float64) int {
		miss := 0
		if !at(x, y, 0, 0) {
			miss++
		}
		for _, o := range alignmentRing {
			if at(x, y, o[0], o[1]) { // light ring
				if miss++; miss == 2 {
					return 0
				}
			}
			if !at(x, y, 2*o[0], 2*o[1]) { // dark ring
				if miss++; miss == 2 {
					return 0
				}
			}
		}
		return 17 - miss
	}

	const steps = 3 // positions per module
	best, bx, by := 0, 0.0, 0.0
	try := func(i, j int) {
		a, b := float64(i)/steps, float64(j)/steps
		x, y := ex+a*ux+b*vx, ey+a*uy+b*vy
		if s := score(x, y); s > best {
			best, bx, by = s, x, y
		}
	}
	ri := 0
	last := int(radii[len(radii)-1] * steps)
	for k := 0; k <= last; k++ {
		// The ring of positions at Chebyshev distance k.
		if k == 0 {
			try(0, 0)
		}
		for i := -k; i <= k && k > 0; i++ {
			try(i, -k)
			try(i, k)
		}
		for j := -k + 1; j < k; j++ {
			try(-k, j)
			try(k, j)
		}
		checkpoint := k == last || k == int(radii[ri]*steps)
		if checkpoint && ri < len(radii)-1 {
			ri++
		}
		if best == 17 || best >= 16 && checkpoint {
			x, y := refineAlignment(dark, bx, by, ux, uy, vx, vy)
			return x, y, true
		}
	}
	return 0, 0, false
}

// alignmentRing lists the offsets of the light ring of an alignment pattern
// in modules; the dark ring is at twice them.
var alignmentRing = [8][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

// refineAlignment moves (x, y) to the middle of the dark center module along
// both module axes, twice.
func refineAlignment(dark func(x, y int) bool, x, y, ux, uy, vx, vy float64) (float64, float64) {
	mid := func(x, y, dx, dy float64) (float64, float64) {
		n := math.Hypot(dx, dy)
		if n == 0 {
			return x, y
		}
		sx, sy := dx/n*0.5, dy/n*0.5 // half-pixel steps
		limit := int(2 * n / 0.5)
		lo, hi := 0, 0
		for lo < limit && dark(int(math.Floor(x-float64(lo+1)*sx)), int(math.Floor(y-float64(lo+1)*sy))) {
			lo++
		}
		for hi < limit && dark(int(math.Floor(x+float64(hi+1)*sx)), int(math.Floor(y+float64(hi+1)*sy))) {
			hi++
		}
		shift := float64(hi-lo) / 2
		return x + shift*sx, y + shift*sy
	}
	for i := 0; i < 2; i++ {
		x, y = mid(x, y, ux, uy)
		x, y = mid(x, y, vx, vy)
	}
	return x, y
}

// mapper maps module coordinates to image coordinates.
type mapper interface {
	apply(u, v float64) (float64, float64)
}

// scaledMapper maps through m, then scales image coordinates by f: from a
// downsampled image, where pixel i covers [i·f, (i+1)·f) of the original,
// to the original.
type scaledMapper struct {
	m mapper
	f float64
}

func (s scaledMapper) apply(u, v float64) (float64, float64) {
	x, y := s.m.apply(u, v)
	return x * s.f, y * s.f
}

// findFinders scans for finder patterns and returns up to maxFinderTriples
// triples that best fit one symbol, best first. The row scan reads bm
// directly; dark (bounds-checked) serves the cross checks.
func findFinders(tm *thresholdMap, inverted bool, dark func(x, y int) bool) ([][3]finderPattern, error) {
	return findFindersStep(tm, inverted, dark, 1)
}

// finderRowStep returns how many rows apart the finder scan runs in an image
// of px pixels. A finder's center block is three modules tall, so every
// third row still crosses it at 1.5 pixels per module, but foreshortening
// and tiny modules leave few rows to spare. Scanning costs grow with the
// image, so small images, which are cheap, are scanned on every row; large
// photos, whose symbols are rarely that small, every second or third. The
// coarser scales of the search are small and scanned on every row.
func finderRowStep(px int) int {
	switch {
	case px >= 4_000_000:
		return 3
	case px >= 1_000_000:
		return 2
	}
	return 1
}

// findFindersStep is findFinders scanning every step-th row; each hit then
// counts for step rows in a candidate's support.
func findFindersStep(tm *thresholdMap, inverted bool, dark func(x, y int) bool, step int) ([][3]finderPattern, error) {
	h := tm.h
	var cands []finderPattern

	add := func(cx, cy, module float64) {
		for i := range cands {
			if math.Abs(cands[i].x-cx) < module && math.Abs(cands[i].y-cy) < module {
				n := float64(cands[i].count)
				cands[i].x = (cands[i].x*n + cx) / (n + 1)
				cands[i].y = (cands[i].y*n + cy) / (n + 1)
				cands[i].moduleSize = (cands[i].moduleSize*n + module) / (n + 1)
				cands[i].count += step
				return
			}
		}
		cands = append(cands, finderPattern{x: cx, y: cy, moduleSize: module, count: step})
	}

	// Each scanned row is read as runs of one class; whenever a dark run
	// ends, the last five runs (dark, light, dark, light, dark) are tested
	// for 1:1:3:1:1. x is the first light pixel after them.
	check := func(s [5]int, x, y int) {
		module, ok := checkFinderRatio(s)
		if !ok {
			return
		}
		cx := float64(x) - float64(s[4]) - float64(s[3]) - float64(s[2])/2
		total := s[0] + s[1] + s[2] + s[3] + s[4]
		// Confirm vertically, then horizontally through the refined
		// center.
		x0 := int(cx + 0.5)
		if dy, mv, ok := crossCheck(dark, x0, y, 0, 1, s[2], total); ok {
			cy := float64(y) + dy
			if dx, mh, ok := crossCheck(dark, x0, int(cy+0.5), 1, 0, s[2], total); ok {
				add(float64(x0)+dx, cy, (module+mv+mh)/3)
			}
		}
	}
	for y := step / 2; y < h; y += step {
		var runs [5]int // the last five runs, oldest first
		n := 0          // runs seen in this row, up to 5
		prev, start := false, 0
		tm.scanRow(y, inverted, func(x int, d bool) {
			// A run of class prev ended at x; d starts the next one.
			length := x - start
			start = x
			if length > 0 {
				copy(runs[:], runs[1:])
				runs[4] = length
				n = min(n+1, 5)
				// The run that ended is dark when the next one is light; a
				// row starts light, so dark runs fill the odd slots.
				if prev && !d && n == 5 {
					check(runs, x, y)
				}
			}
			prev = d
		})
	}

	return selectFinders(cands)
}

// maxFinderTriples bounds how many finder triples are fitted per image.
const maxFinderTriples = 3

// selectFinders returns up to maxFinderTriples candidate triples that look
// most like the finders of one symbol, best first: similar module sizes,
// placed at the corners of a roughly right isosceles triangle at least 14
// modules on a side. Data modules can mimic the 1:1:3:1:1 pattern, and the
// strongest candidates alone are not reliable in large symbols.
func selectFinders(cands []finderPattern) ([][3]finderPattern, error) {
	if len(cands) < 3 {
		return nil, fmt.Errorf("%w: found %d finder patterns", ErrNotFound, len(cands))
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].count > cands[j].count })
	if len(cands) > maxFinderCandidates {
		cands = cands[:maxFinderCandidates]
	}

	type scored struct {
		t     [3]finderPattern
		score float64
	}
	var best []scored
	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			for k := j + 1; k < len(cands); k++ {
				s := finderTripleScore(cands[i], cands[j], cands[k])
				if math.IsInf(s, 1) || len(best) == maxFinderTriples && s >= best[len(best)-1].score {
					continue
				}
				n := sort.Search(len(best), func(x int) bool { return best[x].score > s })
				best = append(best, scored{})
				copy(best[n+1:], best[n:])
				best[n] = scored{[3]finderPattern{cands[i], cands[j], cands[k]}, s}
				if len(best) > maxFinderTriples {
					best = best[:maxFinderTriples]
				}
			}
		}
	}
	if len(best) == 0 {
		return nil, fmt.Errorf("%w: no three finder patterns form a symbol", ErrNotFound)
	}
	out := make([][3]finderPattern, len(best))
	for i, b := range best {
		out[i] = b.t
	}
	return out, nil
}

// maxFinderCandidates bounds the O(n³) triple search.
const maxFinderCandidates = 16

// finderTripleScore rates how well three candidates fit one symbol; lower is
// better and +Inf rejects the triple.
func finderTripleScore(a, b, c finderPattern) float64 {
	lo := min(a.moduleSize, b.moduleSize, c.moduleSize)
	hi := max(a.moduleSize, b.moduleSize, c.moduleSize)
	// Perspective makes the nearer finders larger and the triangle less
	// regular; the tolerances allow about 50 degrees of tilt, and the
	// triple is verified by decoding it.
	if hi > 2*lo {
		return math.Inf(1)
	}
	d := []float64{dist(a, b), dist(b, c), dist(a, c)}
	sort.Float64s(d)
	leg1, leg2, hyp := d[0], d[1], d[2]
	module := (a.moduleSize + b.moduleSize + c.moduleSize) / 3
	// module comes from axis-aligned runs, which overestimate it by up to √2
	// in a rotated symbol.
	if legModules := leg1 / module; legModules < 14*0.8/math.Sqrt2 || legModules > (4*MaxVersion+10)*1.2 {
		return math.Inf(1)
	}
	isosceles := (leg2 - leg1) / leg2
	right := math.Abs(hyp-math.Hypot(leg1, leg2)) / hyp
	if isosceles > 0.4 || right > 0.25 {
		return math.Inf(1)
	}
	// A finder's center block is three modules tall, so the row scan hits a
	// real finder on about three times its module size in rows; data that
	// happens to read 1:1:3:1:1 on a row or two is penalized up to 1, more
	// than any shape term, since perspective distorts shapes but not this.
	support := 0.0
	for _, f := range [3]finderPattern{a, b, c} {
		support = math.Max(support, 1-math.Min(1, float64(f.count)/(1.5*f.moduleSize)))
	}
	return isosceles + right + (hi-lo)/hi + support
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
// coordinate along that axis and the module size of the runs. maxCount is
// the length of the center run seen by the scan that produced the
// candidate, and originalTotal the length of all five. Perspective can
// stretch or compress a finder along one axis to about a third of the
// other, so the runs may be up to twice maxCount and the total within a
// factor of three of originalTotal; the run ratios still have to match.
func crossCheck(dark func(x, y int) bool, cx, cy, dx, dy, maxCount, originalTotal int) (float64, float64, bool) {
	// dark reports false outside the image, so every loop ends: dark runs
	// stop at the border and light runs are bounded by ringMax.
	var s [5]int
	at := func(i int) bool { return dark(cx+i*dx, cy+i*dy) }
	ringMax := 2 * maxCount

	i := 0
	for at(i) && s[2] <= 4*ringMax {
		s[2]++
		i--
	}
	for ; !at(i) && s[1] <= ringMax; i-- {
		s[1]++
	}
	for ; at(i) && s[0] <= ringMax; i-- {
		s[0]++
	}
	if s[0] == 0 || s[1] > ringMax || s[0] > ringMax {
		return 0, 0, false
	}

	i = 1
	for at(i) && s[2] <= 4*ringMax {
		s[2]++
		i++
	}
	for ; !at(i) && s[3] <= ringMax; i++ {
		s[3]++
	}
	for ; at(i) && s[4] <= ringMax; i++ {
		s[4]++
	}
	if s[4] == 0 || s[3] > ringMax || s[4] > ringMax {
		return 0, 0, false
	}

	total := s[0] + s[1] + s[2] + s[3] + s[4]
	if 3*total < originalTotal || total > 3*originalTotal {
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

// handed returns tl with the other two ordered as top-right and bottom-left
// by the sign of their cross product, as orderFinders does.
func handed(tl, a, c finderPattern) (finderPattern, finderPattern, finderPattern) {
	if (a.x-tl.x)*(c.y-tl.y)-(a.y-tl.y)*(c.x-tl.x) >= 0 {
		return tl, a, c
	}
	return tl, c, a
}

func dist(a, b finderPattern) float64 {
	dx := a.x - b.x
	dy := a.y - b.y
	return math.Sqrt(dx*dx + dy*dy)
}

// readVersion reads the two 18-bit version information blocks by sampling
// relative to the finder beside each one, using that finder's module sizes,
// so the result does not depend on the estimated dimension. It returns the
// first block that BCH-corrects to a valid version.
func (g *symbolGeometry) readVersion(dark func(x, y int) bool) (int, bool) {
	sample := func(f finderPattern, cm, rm float64, du, dv func(i int) float64) int {
		bits := 0
		for i := 0; i < 18; i++ {
			px := f.x + du(i)*cm*g.cx + dv(i)*rm*g.rx
			py := f.y + du(i)*cm*g.cy + dv(i)*rm*g.ry
			if dark(int(math.Floor(px)), int(math.Floor(py))) {
				bits |= 1 << i
			}
		}
		return bits
	}
	// Block 1 sits left of the top-right finder: bit i is module
	// (size-11+i%3, i/3), offset (i%3-7, i/3-3) from the finder center.
	if v, ok := correctVersion(sample(g.tr, g.trC, g.trR,
		func(i int) float64 { return float64(i%3 - 7) },
		func(i int) float64 { return float64(i/3 - 3) })); ok {
		return v, true
	}
	// Block 2 is its transpose, above the bottom-left finder.
	return correctVersion(sample(g.bl, g.blC, g.blR,
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
