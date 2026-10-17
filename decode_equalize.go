package qr

import (
	"math"

	"github.com/piglig/go-qr/v2/internal/symbol"
)

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
	pilots := symbol.Pilots(ver)
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
	// Summed-area tables of the terms of the normal equations, refilled
	// each iteration; that of the luminances is the windows'.
	m := (dim + 1) * (dim + 1)
	tables := make([]float64, 6*m)
	sp, sm, spm, smm, slp, slm := tables[:m], tables[m:2*m], tables[2*m:3*m], tables[3*m:4*m], tables[4*m:5*m], tables[5*m:]
	sl := w.sat
	for iter := 0; iter < 3; iter++ {
		copy(label, dark)
		for _, m := range pilots {
			v := 0.0
			if m.Dark != inverted {
				v = 1
			}
			label[int(m.Y)*dim+int(m.X)] = v
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
		sn := dim + 1
		for r := 0; r < dim; r++ {
			var p, mo, pm, mm, lp, lm float64
			for col := 0; col < dim; col++ {
				i := r*dim + col
				l, ar, v := label[i], around[i], vals[i]
				p += l
				mo += ar
				pm += l * ar
				mm += ar * ar
				lp += v * l
				lm += v * ar
				j, k := (r+1)*sn+col+1, r*sn+col+1
				sp[j], sm[j], spm[j], smm[j], slp[j], slm[j] = sp[k]+p, sm[k]+mo, spm[k]+pm, smm[k]+mm, slp[k]+lp, slm[k]+lm
			}
		}
		for r := 0; r < dim; r++ {
			for col := 0; col < dim; col++ {
				i := r*dim + col
				r0, c0, r1, c1 := w.bounds(r, col)
				cnt := float64((r1 - r0) * (c1 - c0))
				p, mo, pm, mm := satSum(sp, sn, r0, c0, r1, c1), satSum(sm, sn, r0, c0, r1, c1), satSum(spm, sn, r0, c0, r1, c1), satSum(smm, sn, r0, c0, r1, c1)
				// Normal equations for (a, b, c); dark² = dark.
				x, ok := solve3([3][3]float64{{cnt, p, mo}, {p, p, pm}, {mo, pm, mm}},
					[3]float64{satSum(sl, sn, r0, c0, r1, c1), satSum(slp, sn, r0, c0, r1, c1), satSum(slm, sn, r0, c0, r1, c1)})
				if !ok || x[1] >= 0 { // no contrast in the window
					rel[i], resid[i] = 0, 0
					continue
				}
				// The decision changes dark, not the labels the fit uses.
				d0 := vals[i] - (x[0] + x[2]*around[i])
				d1 := d0 - x[1]
				if math.Abs(d1) < math.Abs(d0) {
					dark[i], resid[i] = 1, d1*d1
				} else {
					dark[i], resid[i] = 0, d0*d0
				}
				rel[i] = float32(math.Abs(d0*d0 - d1*d1))
			}
		}
	}
	rs := sp // reused for the residuals
	sn := dim + 1
	for r := 0; r < dim; r++ {
		row := 0.0
		for col := 0; col < dim; col++ {
			row += resid[r*dim+col]
			rs[(r+1)*sn+col+1] = rs[r*sn+col+1] + row
		}
	}
	for r := 0; r < dim; r++ {
		for col := 0; col < dim; col++ {
			r0, c0, r1, c1 := w.bounds(r, col)
			v := satSum(rs, sn, r0, c0, r1, c1)/float64((r1-r0)*(c1-c0)) + 1
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

// satSum returns the sum over rows r0 to r1-1 and columns c0 to c1-1 of
// the summed-area table s of a grid of n-1 columns.
func satSum(s []float64, n, r0, c0, r1, c1 int) float64 {
	return s[r1*n+c1] - s[r0*n+c1] - s[r1*n+c0] + s[r0*n+c0]
}

// solve3 solves a·x = y by Cramer's rule.
func solve3(a [3][3]float64, y [3]float64) ([3]float64, bool) {
	det := det3(a[0][0], a[0][1], a[0][2], a[1][0], a[1][1], a[1][2], a[2][0], a[2][1], a[2][2])
	if math.Abs(det) < 1e-9 {
		return [3]float64{}, false
	}
	return [3]float64{
		det3(y[0], a[0][1], a[0][2], y[1], a[1][1], a[1][2], y[2], a[2][1], a[2][2]) / det,
		det3(a[0][0], y[0], a[0][2], a[1][0], y[1], a[1][2], a[2][0], y[2], a[2][2]) / det,
		det3(a[0][0], a[0][1], y[0], a[1][0], a[1][1], y[1], a[2][0], a[2][1], y[2]) / det,
	}, true
}

// det3 returns the determinant of the 3×3 matrix of rows (a, b, c), (d, e,
// f) and (g, h, i).
func det3(a, b, c, d, e, f, g, h, i float64) float64 {
	return a*(e*i-f*h) - b*(d*i-f*g) + c*(d*h-e*g)
}
