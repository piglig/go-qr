package qr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/piglig/go-qr/v2/internal/detect"
)

// robustDecode locates symbols in a binarized image and passes candidate
// module grids to read until one decodes. It returns the first error
// otherwise.
//
// tm thresholds the image at 1/scale of its resolution, inverted for light
// symbols on dark; the symbol is located there, and its modules are read
// from the full-resolution luminance l (lw×lh pixels). With refine, set at
// levels searched before full resolution, finders the level barely
// resolves are located again in regions at full resolution.
// It stops between regions when ctx, which may be nil, is done.
func robustDecode(ctx context.Context, tm *detect.ThresholdMap, inverted bool, scale int, refine bool, l []uint8, lw, lh int, read gridReader) (*DecodeResult, error) {
	s := newSymbolSearch(ctx, tm, inverted, scale, l, lw, lh, read)
	triples, cands, err := detect.FindFindersStep(tm, inverted, detect.DarkIn(tm, inverted), s.step())
	if err != nil && len(cands) < 2 {
		return nil, err
	}
	// Finders that a coarse level barely resolves are located again at
	// full resolution first: their fit at this level is too imprecise to
	// read, and would try every other triple and inferred finder first.
	if refine && scale > 1 {
		var res *DecodeResult
		rerr := s.eachRegion(triples, nil, func(r *DecodeResult) bool { res = r; return false })
		if res != nil || errors.Is(rerr, ErrUnsupported) {
			return res, rerr
		}
		if cerr := s.canceled(); cerr != nil {
			return nil, cerr
		}
	}
	return s.decodeTriples(triples, cands, err, true)
}

// eachRegion locates the symbols of the best triples whose finders a
// coarse level barely resolves, again at full resolution, each within a
// window of the image around it, and passes each symbol read to found
// until it returns false. A coarse level detects a symbol whose modules
// span a pixel or two there, but cannot locate it precisely enough to read
// it; locating it again in a window costs the window's pixels instead of
// the whole image's. Triples for which skip reports true are passed over;
// after maxRegions windows without a symbol, or windows of detect.SearchPixels
// without one, so are the rest: in texture, finders are found everywhere.
func (s *symbolSearch) eachRegion(triples [][3]detect.FinderPattern, skip func([3]detect.FinderPattern) bool, found func(*DecodeResult) bool) error {
	err := fmt.Errorf("%w: no symbol in the regions of the finders", ErrNotFound)
	failed, budget := 0, detect.SearchPixels // pixels of regions without a symbol left
	for _, t := range triples {
		if failed == maxRegions {
			break
		}
		if cerr := s.canceled(); cerr != nil {
			return cerr
		}
		if skip != nil && skip(t) {
			continue
		}
		r, ok := s.region(t)
		if !ok || r.Dx()*r.Dy() > budget {
			continue
		}
		crop := cropLuma(s.l, s.lw, r)
		tm := detect.HybridThresholds(crop, r.Dx(), r.Dy())
		res, rerr := robustDecode(s.ctx, tm, s.inverted, 1, false, crop, r.Dx(), r.Dy(), s.read)
		switch {
		case rerr == nil:
			for i := range res.Corners {
				res.Corners[i] = res.Corners[i].Add(r.Min)
			}
			if res.centers != nil {
				res.centers.shift(r.Min)
			}
			if !found(res) {
				return nil
			}
		case errors.Is(rerr, ErrUnsupported):
			return rerr
		default:
			err = rerr
			failed++
			budget -= r.Dx() * r.Dy()
		}
	}
	return err
}

// region returns the window of the full-resolution image that holds the
// symbol of finder triple t, found at s's level: the parallelogram the
// finders span, whichever of them is the corner one, padded by half a
// finder and the quiet zone. It reports false for finders too large to
// need it, for finders too far apart to be one symbol's, and for windows
// over maxRegionPixels.
func (s *symbolSearch) region(t [3]detect.FinderPattern) (image.Rectangle, bool) {
	module := max(t[0].ModuleSize, t[1].ModuleSize, t[2].ModuleSize)
	if module >= regionModule {
		return image.Rectangle{}, false
	}
	// Finders whose spacing implies a symbol larger than version 40 are
	// not one symbol's, as texture shows; detect.Perspective and module size
	// estimates are allowed regionSlack.
	spacing := max(detect.Dist(t[0], t[1]), detect.Dist(t[0], t[2]), detect.Dist(t[1], t[2]))
	minModule := min(t[0].ModuleSize, t[1].ModuleSize, t[2].ModuleSize)
	if spacing/math.Sqrt2/minModule+7 > (4*MaxVersion+17)*regionSlack {
		return image.Rectangle{}, false
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	extend := func(x, y float64) {
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	for i, c := range t {
		a, b := t[(i+1)%3], t[(i+2)%3]
		extend(c.X, c.Y)
		extend(a.X+b.X-c.X, a.Y+b.Y-c.Y) // the fourth corner, if c is the corner finder
	}
	pad := regionPad * module
	f := float64(s.scale)
	r := image.Rect(int((minX-pad)*f), int((minY-pad)*f), int(math.Ceil((maxX+pad)*f)), int(math.Ceil((maxY+pad)*f)))
	r = r.Intersect(image.Rect(0, 0, s.lw, s.lh))
	if r.Dx() < 21 || r.Dy() < 21 || r.Dx()*r.Dy() > min(maxRegionPixels, s.lw*s.lh/4) {
		return image.Rectangle{}, false
	}
	return r, true
}

// cropLuma copies the pixels of l, w pixels wide, within r.
func cropLuma(l []uint8, w int, r image.Rectangle) []uint8 {
	out := make([]uint8, r.Dx()*r.Dy())
	for y := 0; y < r.Dy(); y++ {
		copy(out[y*r.Dx():], l[(r.Min.Y+y)*w+r.Min.X:(r.Min.Y+y)*w+r.Max.X])
	}
	return out
}

// Coarse levels are located again in regions for finders of fewer than
// regionModule pixels per module there, the regionPad modules around them
// covering half a finder and the quiet zone, until maxRegions regions per
// level hold no symbol.
const (
	regionModule = 3
	regionPad    = 8
	maxRegions   = 3
)

// A region holds at most maxRegionPixels, as two search levels would, and a
// quarter of the image; a symbol that needs more has modules large enough
// at its level to be read there. regionSlack is the margin on the size of
// a symbol in a region.
const (
	maxRegionPixels = 2 * detect.SearchPixels
	regionSlack     = 1.25
)

// symbolSearch locates and reads symbols at one scale and polarity.
type symbolSearch struct {
	ctx      context.Context // nil for none
	tm       *detect.ThresholdMap
	inverted bool
	scale    int
	l        []uint8 // full-resolution luminance, lw×lh
	lw, lh   int
	read     gridReader
}

func newSymbolSearch(ctx context.Context, tm *detect.ThresholdMap, inverted bool, scale int, l []uint8, lw, lh int, read gridReader) *symbolSearch {
	return &symbolSearch{ctx: ctx, tm: tm, inverted: inverted, scale: scale, l: l, lw: lw, lh: lh, read: read}
}

// canceled returns the error of s's context once it is done.
func (s *symbolSearch) canceled() error {
	if s.ctx == nil {
		return nil
	}
	return s.ctx.Err()
}

// step returns how many rows apart the finder scan runs, by the pixels of
// the level it scans: the levels coarser than full resolution are searched
// first, and the half of a 12-megapixel photo has 3 megapixels.
func (s *symbolSearch) step() int {
	return detect.FinderRowStep(s.tm.W * s.tm.H)
}

// readGrid reads and decodes the dim×dim grid that p locates in tm's
// pixels, and records where it lies at full resolution.
func (s *symbolSearch) readGrid(p detect.Mapper, dim int) (*DecodeResult, error) {
	if s.scale > 1 {
		p = detect.ScaledMapper{Mapper: p, Scale: float64(s.scale)}
	}
	res, err := readSoft(detect.ModuleLuma(s.l, s.lw, s.lh, p, dim), dim, s.inverted, s.read)
	locate(res, p.Apply, dim, s.inverted)
	return res, err
}

// decodeTriples fits the finder triples, best first, and decodes the grid
// of the best fit. cands, when not nil, are all finder candidates, from
// which a lost finder may be inferred; firstErr is returned when nothing
// could be fitted. When estimate is set and no fit could measure the
// finders' corners, the grid is estimated from their centers instead.
func (s *symbolSearch) decodeTriples(triples [][3]detect.FinderPattern, cands []detect.FinderPattern, firstErr error, estimate bool) (*DecodeResult, error) {
	dark := detect.DarkIn(s.tm, s.inverted)
	// Fit the best triple with the usual corner assignment. If its timing
	// patterns do not read back well, the triple may be a false one or its
	// top-left finder misjudged, which strong detect.Perspective causes: the other
	// assignments and triples are fitted too, and one of them replaces the
	// first only if its timing patterns read back well. Fitting costs a
	// small fraction of a decode, and only one grid is decoded.
	var (
		model *detect.GridModel
		dim   int
		score = -1.0
		first *detect.SymbolGeometry
	)
search:
	for ti, t := range triples {
		for k := 0; k < 3; k++ {
			if ti > 0 && k > 0 {
				break // other assignments only for the strongest triple
			}
			g, err := detect.NewSymbolGeometryCorner(dark, t, k)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if first == nil {
				first = g
			}
			m, d, sc, ok := g.FitSymbol(dark)
			if ok && (g == first || sc >= detect.GoodTimingScore && sc > score) {
				model, dim, score = m, d, sc
			}
			if score >= detect.GoodTimingScore {
				break search
			}
		}
	}
	// Structural completion: when no triple's timing patterns read back
	// well, a finder may be unreadable, covered by glare, damage or the
	// image edge. Pairs of well-supported finders imply the third, and the
	// hypotheses are fitted and scored like real triples.
	if score < detect.GoodTimingScore && len(cands) >= 2 {
		if m, d, sc, ok := detect.CompleteSymbol(dark, cands); ok && sc > score+detect.CompletionMargin {
			model, dim = m, d
		}
	}
	if model != nil {
		return s.readGrid(model, dim)
	}
	if first == nil || !estimate {
		if firstErr == nil {
			firstErr = fmt.Errorf("%w: finder corners not measured", ErrNotFound)
		}
		return nil, firstErr
	}
	// The finders' corners could not be measured, as with circular finder
	// styles: estimate the fourth point instead.
	g := first
	for _, p := range g.Transforms(dark) {
		res, err := s.readGrid(p, g.Dim)
		if err == nil || errors.Is(err, ErrUnsupported) {
			return res, err
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}
