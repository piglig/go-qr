package qr

import "math"

// Module-space reading.
//
// The classic pipeline thresholds pixels and then samples single pixels at
// module centers. A pixel threshold must pick a neighborhood size in pixels,
// which cannot suit both 2-pixel and 40-pixel modules: ZXing-style 8×8
// blocks see only one class inside a large module, and the threshold
// degenerates. Reading in module space removes the pixel scale altogether:
//
//  1. Each module's luminance is the mean of nine bilinear samples over its
//     central 30%, which also averages out blur and sensor noise.
//  2. A module is dark when it is darker than the mean of the modules
//     around it, within a window measured in modules. Data modules are
//     close to half dark by design (masking balances them), so the window
//     mean tracks the local midpoint between dark and light under uneven
//     lighting, at any scale.

// moduleLuma returns the mean luminance of each module of a dim×dim grid
// located by p, row-major.
func moduleLuma(l []uint8, w, h int, p mapper, dim int) []float64 {
	// Interpolation averages out sensor and compression noise in large
	// modules, but blends neighbors into modules of a pixel or two.
	c := float64(dim) / 2
	x0, y0 := p.apply(c, c)
	x1, y1 := p.apply(c+1, c)
	nearest := math.Hypot(x1-x0, y1-y0) < nearestBelow
	vals := make([]float64, dim*dim)
	offsets := [3]float64{0.35, 0.5, 0.65}
	for r := 0; r < dim; r++ {
		for c := 0; c < dim; c++ {
			s := 0.0
			for _, ov := range offsets {
				for _, ou := range offsets {
					x, y := p.apply(float64(c)+ou, float64(r)+ov)
					if nearest {
						xi, yi := min(max(int(math.Floor(x)), 0), w-1), min(max(int(math.Floor(y)), 0), h-1)
						s += float64(l[yi*w+xi])
					} else {
						s += lumaAt(l, w, h, x, y)
					}
				}
			}
			vals[r*dim+c] = s / 9
		}
	}
	return vals
}

// nearestBelow is the module pitch, in pixels, below which modules are
// sampled at the nearest pixel rather than interpolated.
const nearestBelow = 3

// lumaAt samples luminance bilinearly at image coordinates (x, y), clamping
// to the image.
func lumaAt(l []uint8, w, h int, x, y float64) float64 {
	x -= 0.5
	y -= 0.5
	fx0, fy0 := math.Floor(x), math.Floor(y)
	fx, fy := x-fx0, y-fy0
	x0, y0 := int(fx0), int(fy0)
	x1, y1 := min(max(x0+1, 0), w-1), min(max(y0+1, 0), h-1)
	x0, y0 = min(max(x0, 0), w-1), min(max(y0, 0), h-1)
	a, b := float64(l[y0*w+x0]), float64(l[y0*w+x1])
	c, d := float64(l[y1*w+x0]), float64(l[y1*w+x1])
	return (a*(1-fx)+b*fx)*(1-fy) + (c*(1-fx)+d*fx)*fy
}

// thresholdModules classifies module luminances against the mean of a
// window of modules around each one. inverted reads light modules as dark.
func thresholdModules(vals []float64, dim int, inverted bool) [][]bool {
	// Summed-area table over the module grid.
	n := dim + 1
	sat := make([]float64, n*n)
	for r := 0; r < dim; r++ {
		row := 0.0
		for c := 0; c < dim; c++ {
			row += vals[r*dim+c]
			sat[(r+1)*n+c+1] = sat[r*n+c+1] + row
		}
	}
	radius := max(moduleWindowMin, dim/8)
	modules := make([][]bool, dim)
	grid := make([]bool, dim*dim)
	for r := 0; r < dim; r++ {
		modules[r] = grid[r*dim : (r+1)*dim]
		r0, r1 := max(0, r-radius), min(dim, r+radius+1)
		for c := 0; c < dim; c++ {
			c0, c1 := max(0, c-radius), min(dim, c+radius+1)
			sum := sat[r1*n+c1] - sat[r0*n+c1] - sat[r1*n+c0] + sat[r0*n+c0]
			mean := sum / float64((r1-r0)*(c1-c0))
			modules[r][c] = (vals[r*dim+c] <= mean) != inverted
		}
	}
	return modules
}

// moduleWindowMin is the least radius, in modules, of the thresholding
// window: 3 spans a finder pattern's width, so its rings are compared with
// the light around them.
const moduleWindowMin = 3

// readModules reads the dim×dim grid located by p from luminance.
func readModules(l []uint8, w, h int, p mapper, dim int, inverted bool) [][]bool {
	return thresholdModules(moduleLuma(l, w, h, p, dim), dim, inverted)
}

// rangePyramid holds the darkest and lightest values over 2^k×2^k blocks,
// for k = 1, 2, ...
type rangePyramid struct {
	levels []rangeLevel
}

type rangeLevel struct {
	lo, hi []int
	w, h   int
}

func newRangePyramid(bw, bh int, at func(i int) (lo, hi int)) *rangePyramid {
	base := rangeLevel{make([]int, bw*bh), make([]int, bw*bh), bw, bh}
	for i := range base.lo {
		base.lo[i], base.hi[i] = at(i)
	}
	p := &rangePyramid{}
	cur := base
	for cur.w > 1 || cur.h > 1 {
		nw, nh := (cur.w+1)/2, (cur.h+1)/2
		next := rangeLevel{make([]int, nw*nh), make([]int, nw*nh), nw, nh}
		for y := 0; y < nh; y++ {
			for x := 0; x < nw; x++ {
				lo, hi := 255, 0
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						if yy, xx := 2*y+dy, 2*x+dx; yy < cur.h && xx < cur.w {
							lo, hi = min(lo, cur.lo[yy*cur.w+xx]), max(hi, cur.hi[yy*cur.w+xx])
						}
					}
				}
				next.lo[y*nw+x], next.hi[y*nw+x] = lo, hi
			}
		}
		p.levels = append(p.levels, next)
		cur = next
	}
	return p
}

// contrast returns the range of the smallest window of 3×3 cells, over the
// pyramid levels, around block (bx, by) whose range exceeds minRange.
func (p *rangePyramid) contrast(bx, by, minRange int) (int, int, bool) {
	if p == nil {
		return 0, 0, false
	}
	for k, lv := range p.levels {
		cx, cy := bx>>(k+1), by>>(k+1)
		lo, hi := 255, 0
		for y := max(0, cy-1); y <= min(lv.h-1, cy+1); y++ {
			for x := max(0, cx-1); x <= min(lv.w-1, cx+1); x++ {
				lo, hi = min(lo, lv.lo[y*lv.w+x]), max(hi, lv.hi[y*lv.w+x])
			}
		}
		if hi-lo > minRange {
			return lo, hi, true
		}
	}
	return 0, 0, false
}
