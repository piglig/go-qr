package qr

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Robust localization path: handles rotated, tilted and noisy images that the
// axis-aligned fast path cannot. It detects finder patterns with the classic
// 1:1:3:1:1 run-ratio scan and cross-checks, picks the triples that look
// like one symbol, and samples the module grid through a perspective
// transform. The transform's fourth point, near the bottom-right corner, is
// the hard part. At most two estimates are tried, most precise first:
//
//  1. the bottom-right alignment pattern (version 2 and up), searched for
//     around the position the next estimate predicts;
//  2. the intersection of the outer edges of the top-right and bottom-left
//     finders, which follows the perspective of the symbol, or, when those
//     edges cannot be traced, the parallelogram completion of the three
//     finder centers, which is exact without perspective.
//
// This is the scheme of ZXing and zxing-cpp; quirc and ZBar differ mostly in
// how they find the edges. Trying more candidates, such as the
// parallelogram after a failed edge estimate or other finder triples,
// rescued under 2% of symbols in the robustness sweeps, at the cost of more
// work on images that do not decode.

type finderPattern struct {
	x, y       float64 // center in image space
	moduleSize float64 // from horizontal and vertical runs
	count      int     // number of merged horizontal hits (confidence)
}

// robustDecode locates symbols in a binarized image and passes candidate
// module grids to read until one decodes. It returns the first error
// otherwise.
func robustDecode(bm []bool, w, h int, read func([][]bool) (*DecodeResult, error)) (*DecodeResult, error) {
	dark := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		return bm[y*w+x]
	}
	finders, err := findFinders(bm, dark, w, h)
	if err != nil {
		return nil, err
	}
	g, err := newSymbolGeometry(dark, finders)
	if err != nil {
		return nil, err
	}
	var firstErr error
	for _, p := range g.transforms(dark) {
		res, err := read(sampleGrid(dark, p, g.dim))
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

func newSymbolGeometry(dark func(x, y int) bool, t [3]finderPattern) (*symbolGeometry, error) {
	tl, tr, bl := orderFinders(t[:])
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
// (c, c) near where guess maps it. It scores a template of the pattern's
// center, light ring and dark ring, transformed by the local geometry of
// guess, at positions in windows of growing size, and refines the best
// match to the center of its dark module.
func findAlignment(dark func(x, y int) bool, guess perspective, c float64) (float64, float64, bool) {
	ex, ey := guess.apply(c, c)
	x1, y1 := guess.apply(c+1, c)
	x2, y2 := guess.apply(c, c+1)
	ux, uy := x1-ex, y1-ey // one module along u
	vx, vy := x2-ex, y2-ey // one module along v

	type pt struct{ a, b float64 }
	var light, darkRing []pt
	for _, o := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		light = append(light, pt{o[0], o[1]})
		darkRing = append(darkRing, pt{2 * o[0], 2 * o[1]})
	}
	at := func(x, y, a, b float64) bool {
		return dark(int(math.Floor(x+a*ux+b*vx)), int(math.Floor(y+a*uy+b*vy)))
	}
	score := func(x, y float64) int {
		n := 0
		if at(x, y, 0, 0) {
			n++
		}
		for _, p := range light {
			if !at(x, y, p.a, p.b) {
				n++
			}
		}
		for _, p := range darkRing {
			if at(x, y, p.a, p.b) {
				n++
			}
		}
		return n
	}

	// Search outward ring by ring, in steps of a third of a module, so the
	// usual case, a pattern close to the estimate, ends early. A perfect
	// match ends the search; otherwise the best match with at most one
	// mismatch within the radius is taken.
	const steps = 3
	for _, radius := range []int{4, 8, 16} {
		best, bx, by := 0, 0.0, 0.0
		for k := 0; k <= radius*steps; k++ {
			for i := -k; i <= k; i++ {
				for j := -k; j <= k; j++ {
					if max(abs(i), abs(j)) != k {
						continue
					}
					a, b := float64(i)/steps, float64(j)/steps
					x, y := ex+a*ux+b*vx, ey+a*uy+b*vy
					if s := score(x, y); s > best {
						best, bx, by = s, x, y
					}
				}
			}
			if best == 17 {
				break
			}
		}
		if best >= 16 {
			x, y := refineAlignment(dark, bx, by, ux, uy, vx, vy)
			return x, y, true
		}
	}
	return 0, 0, false
}

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

// sampleGrid reads every module through p by majority vote over five points
// in a cross around its center, which tolerates the edge noise a single
// center sample is sensitive to.
func sampleGrid(dark func(x, y int) bool, p perspective, dim int) [][]bool {
	modules := make([][]bool, dim)
	grid := make([]bool, dim*dim)
	cross := [5][2]float64{{0.5, 0.5}, {0.25, 0.5}, {0.75, 0.5}, {0.5, 0.25}, {0.5, 0.75}}
	for r := 0; r < dim; r++ {
		modules[r] = grid[r*dim : (r+1)*dim]
		for c := 0; c < dim; c++ {
			votes := 0
			for _, o := range cross {
				x, y := p.apply(float64(c)+o[0], float64(r)+o[1])
				if dark(int(math.Floor(x)), int(math.Floor(y))) {
					votes++
				}
			}
			modules[r][c] = votes >= 3
		}
	}
	return modules
}

// findFinders scans for finder patterns and returns the triple that best fits
// one symbol. The row scan reads bm directly; dark (bounds-checked) serves
// the cross checks.
func findFinders(bm []bool, dark func(x, y int) bool, w, h int) ([3]finderPattern, error) {
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
		for x, d := range bm[y*w : (y+1)*w] {
			if d {
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
// of one symbol: similar module sizes, placed at the corners of a roughly
// right isosceles triangle at least 14 modules on a side. Data modules can
// mimic the 1:1:3:1:1 pattern, and the strongest candidates alone are not
// reliable in large symbols.
func selectFinders(cands []finderPattern) ([3]finderPattern, error) {
	if len(cands) < 3 {
		return [3]finderPattern{}, fmt.Errorf("%w: found %d finder patterns", ErrNotFound, len(cands))
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
		return [3]finderPattern{}, fmt.Errorf("%w: no three finder patterns form a symbol", ErrNotFound)
	}
	return [3]finderPattern{cands[best[0]], cands[best[1]], cands[best[2]]}, nil
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
