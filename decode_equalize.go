package qr

import "math"

// classifyModules decides each module of a dim×dim grid of luminances, row
// by row, with the function patterns as pilots: modules whose values are
// known calibrate a model of the image that the threshold of
// thresholdModules does not account for. Over the window of modules around
// each module, it fits by least squares
//
//	luminance = a + b·dark + c·(darkness of the 8 neighbors)
//
// to the pilots with their known values and to the other modules with their
// current decisions: a is the light level, b the contrast, which glare
// lowers, and c the blur that darkens modules next to dark ones. Each module
// is then the value whose predicted luminance is nearer, given its
// neighbors', starting from the local mean threshold and iterating three
// times. It also returns each module's reliability: the log-likelihood
// ratio of the two values, |d0² − d1²| over the residual variance of the
// window. It returns nil if dim is not a symbol's size.
func classifyModules(vals []float64, dim int, inverted bool) ([][]bool, []float32) {
	ver := (dim - 17) / 4
	if ver < MinVersion || ver > MaxVersion || 4*ver+17 != dim {
		return nil, nil
	}
	t := getTemplate(ver)
	n := dim * dim
	w := newModuleWindows(vals, dim)

	// dark holds physical darkness, 1 or 0: inverted symbols have light
	// dark modules.
	dark := make([]float64, n)
	for r := 0; r < dim; r++ {
		for c := 0; c < dim; c++ {
			if vals[r*dim+c] <= w.mean(r, c) {
				dark[r*dim+c] = 1
			}
		}
	}
	label := make([]float64, n) // pilots' known values, others' decisions
	around := make([]float64, n)
	rel := make([]float32, n)
	resid := make([]float64, n)
	a, b, c := make([]float64, n), make([]float64, n), make([]float64, n)
	for iter := 0; iter < 3; iter++ {
		for i := range label {
			label[i] = dark[i]
			if t.pilot[i] {
				label[i] = 0
				if t.pilotDark[i] != inverted {
					label[i] = 1
				}
			}
		}
		// Darkness of the neighbors, the four sides weighing twice the
		// corners; outside the symbol is the light quiet zone.
		for r := 0; r < dim; r++ {
			for col := 0; col < dim; col++ {
				s := 0.0
				for _, d := range neighborWeights {
					if rr, cc := r+d[0], col+d[1]; rr >= 0 && rr < dim && cc >= 0 && cc < dim {
						s += float64(d[2]) * label[rr*dim+cc]
					}
				}
				around[r*dim+col] = s / 12
			}
		}
		sp := newGridSums(dim, func(i int) float64 { return label[i] })
		sm := newGridSums(dim, func(i int) float64 { return around[i] })
		spm := newGridSums(dim, func(i int) float64 { return label[i] * around[i] })
		smm := newGridSums(dim, func(i int) float64 { return around[i] * around[i] })
		sl := newGridSums(dim, func(i int) float64 { return vals[i] })
		slp := newGridSums(dim, func(i int) float64 { return vals[i] * label[i] })
		slm := newGridSums(dim, func(i int) float64 { return vals[i] * around[i] })
		for r := 0; r < dim; r++ {
			for col := 0; col < dim; col++ {
				i := r*dim + col
				r0, c0, r1, c1 := w.bounds(r, col)
				cnt := float64((r1 - r0) * (c1 - c0))
				p, m, pm, mm := sp.sum(r0, c0, r1, c1), sm.sum(r0, c0, r1, c1), spm.sum(r0, c0, r1, c1), smm.sum(r0, c0, r1, c1)
				// Normal equations for (a, b, c); dark² = dark.
				x, ok := solve3([3][3]float64{{cnt, p, m}, {p, p, pm}, {m, pm, mm}},
					[3]float64{sl.sum(r0, c0, r1, c1), slp.sum(r0, c0, r1, c1), slm.sum(r0, c0, r1, c1)})
				if !ok || x[1] >= 0 { // no contrast in the window
					a[i] = math.NaN()
					continue
				}
				a[i], b[i], c[i] = x[0], x[1], x[2]
			}
		}
		for i := range dark {
			if math.IsNaN(a[i]) {
				rel[i], resid[i] = 0, 0
				continue
			}
			d0 := vals[i] - (a[i] + c[i]*around[i])
			d1 := d0 - b[i]
			if math.Abs(d1) < math.Abs(d0) {
				dark[i], resid[i] = 1, d1*d1
			} else {
				dark[i], resid[i] = 0, d0*d0
			}
			rel[i] = float32(math.Abs(d0*d0 - d1*d1))
		}
	}
	rs := newGridSums(dim, func(i int) float64 { return resid[i] })
	for r := 0; r < dim; r++ {
		for col := 0; col < dim; col++ {
			r0, c0, r1, c1 := w.bounds(r, col)
			v := rs.sum(r0, c0, r1, c1)/float64((r1-r0)*(c1-c0)) + 1
			rel[r*dim+col] /= float32(v)
		}
	}
	modules := make([][]bool, dim)
	grid := make([]bool, n)
	for r := range modules {
		modules[r] = grid[r*dim : (r+1)*dim]
		for col := range modules[r] {
			modules[r][col] = (dark[r*dim+col] == 1) != inverted
		}
	}
	return modules, rel
}

// neighborWeights are the offsets and weights of a module's neighbors.
var neighborWeights = [8][3]int{{-1, 0, 2}, {1, 0, 2}, {0, -1, 2}, {0, 1, 2}, {-1, -1, 1}, {-1, 1, 1}, {1, -1, 1}, {1, 1, 1}}

// gridSums is a summed-area table over a dim×dim grid.
type gridSums struct {
	n int
	s []float64
}

func newGridSums(dim int, at func(i int) float64) gridSums {
	n := dim + 1
	t := gridSums{n, make([]float64, n*n)}
	for r := 0; r < dim; r++ {
		row := 0.0
		for c := 0; c < dim; c++ {
			row += at(r*dim + c)
			t.s[(r+1)*n+c+1] = t.s[r*n+c+1] + row
		}
	}
	return t
}

// sum returns the sum over rows r0 to r1-1 and columns c0 to c1-1.
func (t gridSums) sum(r0, c0, r1, c1 int) float64 {
	n := t.n
	return t.s[r1*n+c1] - t.s[r0*n+c1] - t.s[r1*n+c0] + t.s[r0*n+c0]
}

// solve3 solves a·x = y by Cramer's rule.
func solve3(a [3][3]float64, y [3]float64) ([3]float64, bool) {
	det3 := func(m [3][3]float64) float64 {
		return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) + m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	}
	det := det3(a)
	if math.Abs(det) < 1e-9 {
		return [3]float64{}, false
	}
	var x [3]float64
	for k := 0; k < 3; k++ {
		m := a
		for r := 0; r < 3; r++ {
			m[r][k] = y[r]
		}
		x[k] = det3(m) / det
	}
	return x, true
}
