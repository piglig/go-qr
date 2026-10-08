package qr

import "math"

// Structural fit: the robust path's geometry model.
//
// Rather than guessing a fourth corner and letting the decoder judge the
// guess, the symbol's own fixed patterns are measured and fitted:
//
//  1. Each finder pattern is three nested squares. Rays from its center
//     cross the edges of the center block, the light ring and the dark
//     ring; a line fitted to each edge and intersected with its neighbors
//     gives twelve corners per finder, whose module coordinates are known.
//     Because the corners come from the shape of each finder, they carry
//     the local perspective, and no point has to be extrapolated across
//     the symbol.
//  2. A homography is fitted to the 36 corners by normalized least squares
//     (DLT). The corners of the top-right and bottom-left finders depend on
//     the symbol size, so the fit is repeated for nearby versions, and the
//     timing patterns, which must alternate exactly, pick the right one;
//     from version 7 on, the version information confirms it.
//  3. Every alignment pattern is then predicted through the homography,
//     located in a small window and added to the fit.
//  4. Lens distortion bends the grid away from any homography. When enough
//     alignment patterns were found, a quadratic and cubic polynomial in the
//     module coordinates is fitted to the residuals of all correspondences
//     and added to the homography, alternating with refits of the
//     homography. The polynomial needs no distortion center, so it holds for
//     cropped photos too, and extrapolates smoothly to the symbol's edges,
//     where barrel distortion is strongest.
//
// The grid is then sampled once.

// gridModel maps module coordinates to image coordinates: a homography plus
// an optional polynomial correction for lens distortion.
type gridModel struct {
	h     perspective
	scale float64 // module coordinates are divided by it in the polynomial
	px    [polyTerms]float64
	py    [polyTerms]float64
}

// polyTerms are the monomials u², uv, v², u³, u²v, uv², v³; lower orders are
// the homography's.
const polyTerms = 7

func polyBasis(u, v float64) [polyTerms]float64 {
	return [polyTerms]float64{u * u, u * v, v * v, u * u * u, u * u * v, u * v * v, v * v * v}
}

func (m *gridModel) apply(u, v float64) (float64, float64) {
	x, y := m.h.apply(u, v)
	if m.scale == 0 {
		return x, y
	}
	b := polyBasis(u/m.scale, v/m.scale)
	for i, t := range b {
		x += m.px[i] * t
		y += m.py[i] * t
	}
	return x, y
}

// fitDistortion fits the homography and the polynomial correction to cs,
// alternating: the homography to the points with the correction removed,
// then the correction to the residuals of the homography.
func (m *gridModel) fitDistortion(cs []correspondence, dim int) bool {
	m.scale = float64(dim)
	adj := make([]correspondence, len(cs))
	for iter := 0; iter < 3; iter++ {
		var ata [polyTerms][polyTerms]float64
		var atx, aty [polyTerms]float64
		for _, c := range cs {
			x, y := m.h.apply(c.u, c.v)
			b := polyBasis(c.u/m.scale, c.v/m.scale)
			w := c.weight()
			for i := range b {
				for j := range b {
					ata[i][j] += w * b[i] * b[j]
				}
				atx[i] += w * b[i] * (c.x - x)
				aty[i] += w * b[i] * (c.y - y)
			}
		}
		px, ok1 := solveN(ata, atx)
		py, ok2 := solveN(ata, aty)
		if !ok1 || !ok2 {
			m.scale = 0
			return false
		}
		m.px, m.py = px, py
		for i, c := range cs {
			b := polyBasis(c.u/m.scale, c.v/m.scale)
			dx, dy := 0.0, 0.0
			for k, t := range b {
				dx += px[k] * t
				dy += py[k] * t
			}
			adj[i] = correspondence{c.u, c.v, c.x - dx, c.y - dy, c.w}
		}
		h, ok := fitHomography(adj)
		if !ok {
			m.scale = 0
			return false
		}
		m.h = h
	}
	return true
}

// solveN solves the polyTerms×polyTerms system a·x = b by Gaussian
// elimination with partial pivoting.
func solveN(a [polyTerms][polyTerms]float64, b [polyTerms]float64) ([polyTerms]float64, bool) {
	const n = polyTerms
	for col := 0; col < n; col++ {
		p := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[p][col]) {
				p = r
			}
		}
		if math.Abs(a[p][col]) < 1e-12 {
			return b, false
		}
		a[col], a[p] = a[p], a[col]
		b[col], b[p] = b[p], b[col]
		for r := col + 1; r < n; r++ {
			f := a[r][col] / a[col][col]
			for c := col; c < n; c++ {
				a[r][c] -= f * a[col][c]
			}
			b[r] -= f * b[col]
		}
	}
	var x [n]float64
	for r := n - 1; r >= 0; r-- {
		s := b[r]
		for c := r + 1; c < n; c++ {
			s -= a[r][c] * x[c]
		}
		x[r] = s / a[r][r]
	}
	return x, true
}

// correspondence pairs module coordinates with an image position, weighted
// in the least-squares fits; zero weight counts as one.
type correspondence struct {
	u, v, x, y float64
	w          float64
}

func (c correspondence) weight() float64 {
	if c.w == 0 {
		return 1
	}
	return c.w
}

// alignmentWeight is the fit weight of an alignment pattern center. The
// twelve corners of a finder crowd one corner of the symbol, while each
// alignment pattern alone constrains its region, the bottom-right one the
// whole far corner; equal weights would let the finders dominate there.
const alignmentWeight = 3

// fitHomography fits the homography that maps module coordinates to image
// coordinates by linear least squares, after normalizing both point sets
// (Hartley, "In defense of the eight-point algorithm", 1997).
func fitHomography(cs []correspondence) (perspective, bool) {
	if len(cs) < 4 {
		return perspective{}, false
	}
	norm := func(get func(c correspondence) (float64, float64)) (mx, my, s float64) {
		for _, c := range cs {
			x, y := get(c)
			mx += x
			my += y
		}
		n := float64(len(cs))
		mx, my = mx/n, my/n
		d := 0.0
		for _, c := range cs {
			x, y := get(c)
			d += math.Hypot(x-mx, y-my)
		}
		if d == 0 {
			return mx, my, 0
		}
		return mx, my, math.Sqrt2 * n / d
	}
	mu, mv, su := norm(func(c correspondence) (float64, float64) { return c.u, c.v })
	mx, my, sx := norm(func(c correspondence) (float64, float64) { return c.x, c.y })
	if su == 0 || sx == 0 {
		return perspective{}, false
	}

	// Normal equations of the 2n×8 system with h33 = 1.
	var ata [8][8]float64
	var atb [8]float64
	add := func(row [8]float64, b, w float64) {
		for i := 0; i < 8; i++ {
			if row[i] == 0 {
				continue
			}
			for j := 0; j < 8; j++ {
				ata[i][j] += w * row[i] * row[j]
			}
			atb[i] += w * row[i] * b
		}
	}
	for _, c := range cs {
		u, v := (c.u-mu)*su, (c.v-mv)*su
		x, y := (c.x-mx)*sx, (c.y-my)*sx
		w := c.weight()
		add([8]float64{u, v, 1, 0, 0, 0, -u * x, -v * x}, x, w)
		add([8]float64{0, 0, 0, u, v, 1, -u * y, -v * y}, y, w)
	}
	h, ok := solve8(ata, atb)
	if !ok {
		return perspective{}, false
	}
	hn := perspective{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7]}
	toNorm := perspective{a: su, c: -su * mu, e: su, f: -su * mv}
	fromNorm := perspective{a: 1 / sx, c: mx, e: 1 / sx, f: my}
	return fromNorm.compose(hn.compose(toNorm)), true
}

// solve8 solves a·x = b by Gaussian elimination with partial pivoting.
func solve8(a [8][8]float64, b [8]float64) ([8]float64, bool) {
	for col := 0; col < 8; col++ {
		p := col
		for r := col + 1; r < 8; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[p][col]) {
				p = r
			}
		}
		if math.Abs(a[p][col]) < 1e-12 {
			return b, false
		}
		a[col], a[p] = a[p], a[col]
		b[col], b[p] = b[p], b[col]
		for r := col + 1; r < 8; r++ {
			f := a[r][col] / a[col][col]
			for c := col; c < 8; c++ {
				a[r][c] -= f * a[col][c]
			}
			b[r] -= f * b[col]
		}
	}
	var x [8]float64
	for r := 7; r >= 0; r-- {
		s := b[r]
		for c := r + 1; c < 8; c++ {
			s -= a[r][c] * x[c]
		}
		x[r] = s / a[r][r]
	}
	return x, true
}

// finderCorners holds the corners of a finder's three nested squares in
// image space, indexed [level][corner] with corners in the order top-left,
// top-right, bottom-right, bottom-left of the finder's local axes. Level 0
// is the outer edge (offsets 0 and 7 modules), 1 the inner edge of the dark
// ring (1 and 6), 2 the edge of the center block (2 and 5).
type finderCorners struct {
	pt [3][4][2]float64
	ok [3][4]bool
}

// traceFinderCorners measures the nested squares of finder f, whose local
// column and row directions are (cx, cy) and (rx, ry) with module sizes cm
// and rm along them.
func traceFinderCorners(dark func(x, y int) bool, f finderPattern, cx, cy, cm, rx, ry, rm float64) finderCorners {
	// sides: normal direction, tangent direction and their module sizes,
	// in the order top (-r), right (+c), bottom (+r), left (-c).
	type side struct{ nx, ny, nm, tx, ty, tm float64 }
	sides := [4]side{
		{-rx, -ry, rm, cx, cy, cm},
		{cx, cy, cm, rx, ry, rm},
		{rx, ry, rm, cx, cy, cm},
		{-cx, -cy, cm, rx, ry, rm},
	}
	var lines [3][4]line
	var have [3][4]bool
	var buf [3][24][2]float64 // 9 center rays and 11 outer rays fit
	for si, sd := range sides {
		pts := [3][][2]float64{buf[0][:0], buf[1][:0], buf[2][:0]}
		// Rays from the center block cross all three edges: dark to light
		// (level 2), light to dark (level 1), dark to light (level 0).
		for s := -1.0; s <= 1.0; s += 0.25 {
			x0 := f.x + s*sd.tm*sd.tx
			y0 := f.y + s*sd.tm*sd.ty
			if !dark(int(math.Floor(x0)), int(math.Floor(y0))) {
				continue
			}
			e, n := rayEdges(dark, x0, y0, sd.nx, sd.ny, 4.5*sd.nm, 3)
			for i := 0; i < n; i++ {
				pts[2-i] = append(pts[2-i], e[i])
			}
		}
		// The outer edge is also traced from the dark ring along most of
		// its length, which steadies its direction.
		for s := -2.5; s <= 2.5; s += 0.5 {
			x0 := f.x + 3*sd.nm*sd.nx + s*sd.tm*sd.tx
			y0 := f.y + 3*sd.nm*sd.ny + s*sd.tm*sd.ty
			if !dark(int(math.Floor(x0)), int(math.Floor(y0))) {
				continue
			}
			if e, n := rayEdges(dark, x0, y0, sd.nx, sd.ny, 2*sd.nm, 1); n == 1 {
				pts[0] = append(pts[0], e[0])
			}
		}
		for level := range pts {
			if len(pts[level]) < 4 {
				continue
			}
			l, ok := fitLine(pts[level])
			// The edges of a square finder are straight; round finder
			// styles bend them by up to a module, and their "corners" would
			// mislead the fit.
			if ok && l.rmsDistance(pts[level]) < math.Max(maxEdgeDeviation*sd.nm, minEdgeTolerance) {
				lines[level][si], have[level][si] = l, true
			}
		}
	}

	var fc finderCorners
	// Corner k lies between sides k-1 and k: top-left between left and top,
	// top-right between top and right, and so on.
	for level := 0; level < 3; level++ {
		for k := 0; k < 4; k++ {
			a, b := (k+3)%4, k
			if !have[level][a] || !have[level][b] {
				continue
			}
			x, y, ok := lines[level][a].intersect(lines[level][b])
			// A corner must lie near where the module sizes put it.
			ex := f.x + cornerSign[k][0]*(3.5-float64(level))*cm*cx + cornerSign[k][1]*(3.5-float64(level))*rm*rx
			ey := f.y + cornerSign[k][0]*(3.5-float64(level))*cm*cy + cornerSign[k][1]*(3.5-float64(level))*rm*ry
			if ok && math.Hypot(x-ex, y-ey) < 1.5*math.Max(cm, rm) {
				fc.pt[level][k] = [2]float64{x, y}
				fc.ok[level][k] = true
			}
		}
	}
	return fc
}

// rayEdges walks from (x0, y0) along the unit direction (dx, dy) for up to
// maxLen pixels, visiting each pixel the ray passes through (Amanatides and
// Woo, "A fast voxel traversal algorithm", 1987), and returns where the ray
// crosses between dark and light the first n (at most 3) times, and how
// many it found. The crossings lie exactly on pixel boundaries.
func rayEdges(dark func(x, y int) bool, x0, y0, dx, dy, maxLen float64, n int) ([3][2]float64, int) {
	ix, iy := int(math.Floor(x0)), int(math.Floor(y0))
	stepX, stepY := 1, 1
	tMaxX, tMaxY := math.Inf(1), math.Inf(1)
	tDeltaX, tDeltaY := math.Inf(1), math.Inf(1)
	if dx < 0 {
		stepX = -1
	}
	if dy < 0 {
		stepY = -1
	}
	if dx != 0 {
		tDeltaX = math.Abs(1 / dx)
		next := float64(ix + max(stepX, 0))
		tMaxX = (next - x0) / dx
	}
	if dy != 0 {
		tDeltaY = math.Abs(1 / dy)
		next := float64(iy + max(stepY, 0))
		tMaxY = (next - y0) / dy
	}
	var out [3][2]float64
	found := 0
	prev := dark(ix, iy)
	for found < n {
		var t float64
		if tMaxX < tMaxY {
			t = tMaxX
			tMaxX += tDeltaX
			ix += stepX
		} else {
			t = tMaxY
			tMaxY += tDeltaY
			iy += stepY
		}
		if t > maxLen {
			break
		}
		if d := dark(ix, iy); d != prev {
			prev = d
			out[found] = [2]float64{x0 + t*dx, y0 + t*dy}
			found++
		}
	}
	return out, found
}

// maxEdgeDeviation is the largest RMS distance, in modules, of traced edge
// points from their fitted line for the edge to count as straight, and
// minEdgeTolerance the least, in pixels: the crossings sit on pixel
// boundaries, which scatter a straight edge by a fraction of a pixel.
const (
	maxEdgeDeviation = 0.2
	minEdgeTolerance = 0.8
)

// cornerSign gives the direction of each finder corner from its center along
// the column and row axes.
var cornerSign = [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}

// finderCorrespondences returns the module coordinates of the measured
// corners of a finder whose top-left module corner is at (ou, ov).
func finderCorrespondences(fc finderCorners, ou, ov float64) []correspondence {
	cs := make([]correspondence, 0, 12)
	for level := 0; level < 3; level++ {
		lo, hi := float64(level), float64(7-level)
		mod := [4][2]float64{{lo, lo}, {hi, lo}, {hi, hi}, {lo, hi}}
		for k := 0; k < 4; k++ {
			if fc.ok[level][k] {
				cs = append(cs, correspondence{ou + mod[k][0], ov + mod[k][1], fc.pt[level][k][0], fc.pt[level][k][1], 1})
			}
		}
	}
	return cs
}

// minFinderCorners is the number of measured corners each finder must
// contribute for the structural fit; fewer fall back to the finder centers.
const minFinderCorners = 4

// fitSymbol fits a grid model to the symbol located by g and returns it with
// the symbol size and the fraction of timing pattern modules that read back
// correctly through it. It reports false when the finders' corners cannot
// be measured, such as with circular finder styles.
func (g *symbolGeometry) fitSymbol(dark func(x, y int) bool) (*gridModel, int, float64, bool) {
	fcs := [3]finderCorners{
		traceFinderCorners(dark, g.tl, g.cx, g.cy, g.tlC, g.rx, g.ry, g.tlR),
		traceFinderCorners(dark, g.tr, g.cx, g.cy, g.trC, g.rx, g.ry, g.trR),
		traceFinderCorners(dark, g.bl, g.cx, g.cy, g.blC, g.rx, g.ry, g.blR),
	}
	for _, fc := range fcs {
		n := 0
		for level := range fc.ok {
			for _, ok := range fc.ok[level] {
				if ok {
					n++
				}
			}
		}
		if n < minFinderCorners {
			return nil, 0, 0, false
		}
	}
	fit := func(dim int) (perspective, []correspondence, bool) {
		d := float64(dim)
		cs := finderCorrespondences(fcs[0], 0, 0)
		cs = append(cs, finderCorrespondences(fcs[1], d-7, 0)...)
		cs = append(cs, finderCorrespondences(fcs[2], 0, d-7)...)
		h, ok := fitHomography(cs)
		return h, cs, ok
	}

	// Pick the version whose timing patterns read back best, starting
	// with the estimate: a wrong size puts the timing pattern out of phase
	// within a few modules, so an estimate that reads back well is right.
	v0 := (g.dim - 17) / 4
	bestV, bestScore := 0, -1.0
	var bestH perspective
	var bestCS []correspondence
	for _, dv := range [5]int{0, -1, 1, -2, 2} {
		v := v0 + dv
		if v < MinVersion || v > MaxVersion {
			continue
		}
		h, cs, ok := fit(4*v + 17)
		if !ok {
			continue
		}
		if s := timingScore(dark, h, 4*v+17); s > bestScore {
			bestV, bestScore, bestH, bestCS = v, s, h, cs
		}
		if dv == 0 && bestScore >= sureTimingScore {
			break
		}
	}
	if bestScore < 0 {
		return nil, 0, 0, false
	}
	if bestV >= 7 {
		if v, ok := readVersionThrough(dark, bestH, 4*bestV+17); ok && v != bestV {
			if h, cs, ok := fit(4*v + 17); ok {
				bestV, bestH, bestCS = v, h, cs
			}
		}
	}
	dim := 4*bestV + 17
	model := &gridModel{h: bestH}

	// Grow the fit with the alignment patterns; a second pass looks again
	// for those missed, through the homography refitted to the first.
	pos := alignmentPositions(bestV)
	if len(pos) == 0 {
		return model, dim, bestScore, true
	}
	n := len(pos)
	found := make([]bool, n*n)
	cs := bestCS
	for pass := 0; pass < 2; pass++ {
		added := false
		for j := 0; j < n; j++ {
			for i := 0; i < n; i++ {
				if isFinderSlot(i, j, n) || found[j*n+i] {
					continue
				}
				u, v := float64(pos[i])+0.5, float64(pos[j])+0.5
				if x, y, ok := locateAlignment(dark, model.h, u, v, 2.5); ok {
					found[j*n+i] = true
					cs = append(cs, correspondence{u, v, x, y, alignmentWeight})
					added = true
				}
			}
		}
		if !added {
			break
		}
		if h, ok := fitHomography(cs); ok {
			model.h = h
		}
	}

	// Lens distortion: fit the polynomial correction when the alignment
	// patterns spread over the symbol constrain it.
	nfound := 0
	for _, f := range found {
		if f {
			nfound++
		}
	}
	// Keep the correction only if the fixed patterns read back better with
	// it: with small modules, noisy alignment centers make the cubic terms
	// bend the grid instead of straightening it.
	if nfound >= minDistortionPoints {
		plain := *model
		if model.fitDistortion(cs, dim) && structureScore(dark, model, bestV) <= structureScore(dark, &plain, bestV) {
			*model = plain
		}
	}
	return model, dim, bestScore, true
}

// structureScore counts the timing and alignment pattern modules that read
// back as they must through m.
func structureScore(dark func(x, y int) bool, m mapper, version int) int {
	dim := 4*version + 17
	isDark := func(u, v float64) bool {
		x, y := m.apply(u, v)
		return dark(int(math.Floor(x)), int(math.Floor(y)))
	}
	n := 0
	for i := 8; i <= dim-9; i++ {
		want := i%2 == 0
		if isDark(float64(i)+0.5, 6.5) == want {
			n++
		}
		if isDark(6.5, float64(i)+0.5) == want {
			n++
		}
	}
	pos := alignmentPositions(version)
	for j := range pos {
		for i := range pos {
			if isFinderSlot(i, j, len(pos)) {
				continue
			}
			for dv := -2; dv <= 2; dv++ {
				for du := -2; du <= 2; du++ {
					want := max(abs(du), abs(dv)) != 1
					if isDark(float64(pos[i]+du)+0.5, float64(pos[j]+dv)+0.5) == want {
						n++
					}
				}
			}
		}
	}
	return n
}

// minDistortionPoints is the number of alignment patterns needed to fit the
// lens distortion correction; with fewer, the cubic terms are poorly
// constrained in the symbol's interior.
const minDistortionPoints = 4

// sureTimingScore is the timing pattern agreement at which the estimated
// version is taken without fitting its neighbors.
const sureTimingScore = 0.95

// isFinderSlot reports whether alignment grid position (i, j) is covered by
// a finder pattern.
func isFinderSlot(i, j, n int) bool {
	return i == 0 && j == 0 || i == n-1 && j == 0 || i == 0 && j == n-1
}

// timingScore returns the fraction of timing pattern modules (row and
// column 6 between the finders) that read as they must: dark at even
// positions.
func timingScore(dark func(x, y int) bool, h perspective, dim int) float64 {
	ok, total := 0, 0
	for i := 8; i <= dim-9; i++ {
		want := i%2 == 0
		for _, uv := range [2][2]float64{{float64(i) + 0.5, 6.5}, {6.5, float64(i) + 0.5}} {
			x, y := h.apply(uv[0], uv[1])
			if dark(int(math.Floor(x)), int(math.Floor(y))) == want {
				ok++
			}
			total++
		}
	}
	return float64(ok) / float64(total)
}

// readVersionThrough reads the version information blocks through h and
// returns the first that BCH-corrects to a valid version.
func readVersionThrough(dark func(x, y int) bool, h perspective, dim int) (int, bool) {
	read := func(mod func(i int) (int, int)) (int, bool) {
		bits := 0
		for i := 0; i < 18; i++ {
			c, r := mod(i)
			x, y := h.apply(float64(c)+0.5, float64(r)+0.5)
			if dark(int(math.Floor(x)), int(math.Floor(y))) {
				bits |= 1 << i
			}
		}
		return correctVersion(bits)
	}
	if v, ok := read(func(i int) (int, int) { return dim - 11 + i%3, i / 3 }); ok {
		return v, true
	}
	return read(func(i int) (int, int) { return i / 3, dim - 11 + i%3 })
}

// locateAlignment finds the alignment pattern centered at module (u, v)
// within radius modules of where h maps it.
func locateAlignment(dark func(x, y int) bool, h perspective, u, v, radius float64) (float64, float64, bool) {
	ex, ey := h.apply(u, v)
	x1, y1 := h.apply(u+1, v)
	x2, y2 := h.apply(u, v+1)
	return searchAlignment(dark, ex, ey, x1-ex, y1-ey, x2-ex, y2-ey, []float64{radius})
}
