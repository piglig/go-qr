package go_qr

import (
	"fmt"
	"image"
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

// robustSample binarizes, locates finders, and samples the grid via affine.
func robustSample(img image.Image) ([][]bool, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 21 || h < 21 {
		return nil, fmt.Errorf("%w: image too small", ErrNoQRCode)
	}
	bitmap := binarizeFast(img, b, w, h)
	dark := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		return bitmap[y*w+x]
	}

	finders, err := findFinders(dark, w, h)
	if err != nil {
		return nil, err
	}

	tl, tr, bl := orderFinders(finders)
	moduleSize := (tl.moduleSize + tr.moduleSize + bl.moduleSize) / 3
	if moduleSize <= 0 {
		return nil, fmt.Errorf("%w: bad module size", ErrNoQRCode)
	}

	dimension, err := computeDimension(tl, tr, bl, moduleSize)
	if err != nil {
		return nil, err
	}
	ver := (dimension - 17) / 4
	if ver < MinVersion || ver > MaxVersion {
		return nil, fmt.Errorf("%w: bad dimension %d", ErrNoQRCode, dimension)
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

	add := func(cx, cy, module float64, total int) {
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
							if cy, ok := crossCheckVertical(dark, h, int(cx+0.5), y, s[2], total); ok {
								add(cx, cy, module, total)
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

	if len(cands) < 3 {
		return nil, fmt.Errorf("%w: found %d finder patterns", ErrNoQRCode, len(cands))
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].count > cands[j].count })
	return cands[:3], nil
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

// crossCheckVertical confirms a horizontal candidate by scanning vertically
// through (centerX, startY), returning the refined center y.
func crossCheckVertical(dark func(x, y int) bool, h, centerX, startY, maxCount, originalTotal int) (float64, bool) {
	var s [5]int
	i := startY
	for i >= 0 && dark(centerX, i) {
		s[2]++
		i--
	}
	if i < 0 {
		return 0, false
	}
	for i >= 0 && !dark(centerX, i) && s[1] <= maxCount {
		s[1]++
		i--
	}
	if i < 0 || s[1] > maxCount {
		return 0, false
	}
	for i >= 0 && dark(centerX, i) && s[0] <= maxCount {
		s[0]++
		i--
	}
	if s[0] > maxCount {
		return 0, false
	}

	i = startY + 1
	for i < h && dark(centerX, i) {
		s[2]++
		i++
	}
	if i >= h {
		return 0, false
	}
	for i < h && !dark(centerX, i) && s[3] <= maxCount {
		s[3]++
		i++
	}
	if i >= h || s[3] > maxCount {
		return 0, false
	}
	for i < h && dark(centerX, i) && s[4] <= maxCount {
		s[4]++
		i++
	}
	if s[4] > maxCount {
		return 0, false
	}

	total := s[0] + s[1] + s[2] + s[3] + s[4]
	if 5*abs(total-originalTotal) >= 2*originalTotal {
		return 0, false
	}
	if _, ok := checkFinderRatio(s); !ok {
		return 0, false
	}
	return float64(i) - float64(s[4]) - float64(s[3]) - float64(s[2])/2, true
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
		return 0, fmt.Errorf("%w: estimated dimension %.1f maps to version %d", ErrNoQRCode, raw, ver)
	}
	return ver*4 + 17, nil
}
