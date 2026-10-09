package qr

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sort"
)

// WithMaxSymbols makes DecodeAll stop after n symbols, which saves time when
// only the first few are needed. n must be at least 1. Decode ignores it.
// Since v2.6.
func WithMaxSymbols(n int) DecodeOption {
	return func(c *decodeConfig) { c.maxSymbols, c.maxSymbolsSet = n, true }
}

// maxDecodeAllSymbols bounds the symbols DecodeAll returns, and with them
// its running time, whatever WithMaxSymbols asks for.
const maxDecodeAllSymbols = 256

// DecodeAll finds and decodes every QR Code in img, such as the labels in a
// photo of a shelf or the symbols of a structured append sequence, which
// JoinStructuredAppend then joins. It returns the symbols in reading order:
// rows from top to bottom, each from left to right, by the symbols'
// centers. Each symbol is returned once, and identical symbols at
// different places are all returned.
//
// The error is nil when at least one symbol decodes. Otherwise it is the
// error Decode would return for img. Since v2.6.
func DecodeAll(img image.Image, opts ...DecodeOption) ([]*DecodeResult, error) {
	var cfg decodeConfig
	for _, o := range opts {
		o(&cfg)
	}
	limit := maxDecodeAllSymbols
	if cfg.maxSymbolsSet {
		if cfg.maxSymbols < 1 {
			return nil, fmt.Errorf("%w: WithMaxSymbols(%d): want at least 1", ErrInvalidArgument, cfg.maxSymbols)
		}
		limit = min(limit, cfg.maxSymbols)
	}
	found, err := searchAll(img, cfg, limit)
	if len(found) == 0 {
		return nil, err
	}
	for _, res := range found {
		for i := range res.Corners {
			res.Corners[i] = res.Corners[i].Add(img.Bounds().Min)
		}
	}
	return readingOrder(found), nil
}

// searchAll is searchImage for every symbol: the robust path, at each scale
// and polarity that searchImage tries, decodes every triple of finder
// candidates that may form a symbol not found yet. A pass that finds
// nothing that way falls back to searchImage's own search, which can also
// infer a lost finder.
func searchAll(img image.Image, cfg decodeConfig, limit int) ([]*DecodeResult, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 21 || h < 21 {
		return nil, fmt.Errorf("%w: image too small (%dx%d)", ErrNotFound, w, h)
	}
	l := toLuma(img)
	threshold, lo, hi := otsuThreshold(l)
	if hi-lo < minContrast {
		return nil, fmt.Errorf("%w: image has no contrast", ErrNotFound)
	}

	var firstErr error
	note := func(err error) {
		if firstErr == nil || errors.Is(firstErr, ErrNotFound) && !errors.Is(err, ErrNotFound) {
			firstErr = err
		}
	}
	var found []*DecodeResult
	// add keeps res unless it is a symbol already found, which another
	// scale or polarity can find again: one whose center lies within it.
	add := func(res *DecodeResult) {
		cx, cy := quadCenter(res.Corners)
		for _, f := range found {
			if inQuad(f.Corners, cx, cy) {
				return
			}
			if fx, fy := quadCenter(f.Corners); inQuad(res.Corners, fx, fy) {
				return
			}
		}
		found = append(found, res)
	}

	// A crisp image of a single symbol, as the fast path reads, holds no
	// other symbol: the fast path needs the dark pixels' bounding box to be
	// that symbol.
	for _, inverted := range []bool{false, true} {
		grid, box, err := fastSample(l, w, h, threshold, inverted)
		if err == nil {
			var res *DecodeResult
			if res, err = decodeGrid(grid); err == nil {
				d := float64(len(grid))
				locate(res, func(u, v float64) (float64, float64) {
					return float64(box.Min.X) + u/d*float64(box.Dx()), float64(box.Min.Y) + v/d*float64(box.Dy())
				}, len(grid), inverted)
				return []*DecodeResult{res}, nil
			}
		}
		note(err)
	}
	if cfg.fastPathOnly {
		return nil, firstErr
	}

	pass := func(tm *thresholdMap, inverted bool, scale int) {
		s := newSymbolSearch(tm, inverted, scale, l, w, h, decodeGrid)
		triples, cands, err := findFindersStep(tm, inverted, darkIn(tm, inverted), s.step())
		before := len(found)
		s.decodeEveryTriple(cands, &found, add, limit)
		if len(found) > before {
			return
		}
		if err != nil && len(cands) < 2 {
			note(err)
			return
		}
		if res, err := s.decodeTriples(triples, cands, err, true); err == nil {
			add(res)
		} else {
			note(err)
		}
	}
	adaptive := hybridThresholds(l, w, h)
	for _, inverted := range []bool{false, true} {
		if len(found) < limit {
			pass(adaptive, inverted, 1)
		}
	}
	small, sw, sh := l, w, h
	for scale := 2; min(w, h)/scale >= minScaledSide && len(found) < limit; scale *= 2 {
		small, sw, sh = halve(small, sw, sh)
		pass(hybridThresholds(small, sw, sh), false, scale)
	}
	if len(found) > limit {
		found = found[:limit]
	}
	return found, firstErr
}

const (
	// maxEveryTripleCandidates bounds the finder candidates, the best
	// supported first, that decodeEveryTriple pairs up.
	maxEveryTripleCandidates = 512
	// tripleNeighbors is how many of its nearest compatible candidates each
	// candidate is paired with. A symbol's other two finders are among
	// the nearest, even in a dense grid of symbols.
	tripleNeighbors = 8
	// maxTripleFailures bounds the triples that fail to decode per pass,
	// which in an image of texture or finder-like patterns is most of them.
	maxTripleFailures = 32
)

// decodeEveryTriple decodes, best first, each triple of finder candidates
// that may be a symbol, skipping triples with a finder of a symbol already
// in *found or centered inside one. It passes each new symbol to add and
// stops at limit symbols.
func (s *symbolSearch) decodeEveryTriple(cands []finderPattern, found *[]*DecodeResult, add func(*DecodeResult), limit int) {
	if len(cands) < 3 {
		return
	}
	cands = append([]finderPattern(nil), cands...)
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].count > cands[j].count })
	if len(cands) > maxEveryTripleCandidates {
		cands = cands[:maxEveryTripleCandidates]
	}

	type scored struct {
		i, j, k int
		score   float64
	}
	var triples []scored
	seen := map[[3]int]bool{}
	type neighbor struct {
		j int
		d float64
	}
	var near []neighbor
	for i, a := range cands {
		near = near[:0]
		for j, b := range cands {
			if j == i || max(a.moduleSize, b.moduleSize) > 2*min(a.moduleSize, b.moduleSize) {
				continue
			}
			module := (a.moduleSize + b.moduleSize) / 2
			d := dist(a, b)
			if d < 14*0.8/math.Sqrt2*module || d > (4*MaxVersion+10)*1.2*math.Sqrt2*module {
				continue
			}
			near = append(near, neighbor{j, d})
		}
		sort.Slice(near, func(x, y int) bool { return near[x].d < near[y].d })
		if len(near) > tripleNeighbors {
			near = near[:tripleNeighbors]
		}
		for x := range near {
			for y := x + 1; y < len(near); y++ {
				key := [3]int{i, near[x].j, near[y].j}
				sort.Ints(key[:])
				if seen[key] {
					continue
				}
				seen[key] = true
				if sc := finderTripleScore(cands[key[0]], cands[key[1]], cands[key[2]]); !math.IsInf(sc, 1) {
					triples = append(triples, scored{key[0], key[1], key[2], sc})
				}
			}
		}
	}
	sort.SliceStable(triples, func(x, y int) bool { return triples[x].score < triples[y].score })

	f := float64(s.scale)
	used := make([]bool, len(cands))
	// covered reports whether candidate c, in tm's pixels, lies inside a
	// symbol found so far.
	covered := func(x, y float64) bool {
		for _, r := range *found {
			if inQuad(r.Corners, x*f, y*f) {
				return true
			}
		}
		return false
	}
	failures := 0
	for _, t := range triples {
		if len(*found) >= limit || failures >= maxTripleFailures {
			return
		}
		if used[t.i] || used[t.j] || used[t.k] {
			continue
		}
		a, b, c := cands[t.i], cands[t.j], cands[t.k]
		if covered(a.x, a.y) || covered(b.x, b.y) || covered(c.x, c.y) {
			used[t.i], used[t.j], used[t.k] = true, true, true
			continue
		}
		// Grids estimated from finder centers decode no symbols here that
		// the fallback pass would not, and false triples in texture would
		// each try several.
		res, err := s.decodeTriples([][3]finderPattern{{a, b, c}}, nil, nil, false)
		if err != nil || res == nil {
			failures++
			continue
		}
		n := len(*found)
		add(res)
		if len(*found) == n {
			continue // found before, at another scale or polarity
		}
		for i, c := range cands {
			if !used[i] && inQuad(res.Corners, c.x*f, c.y*f) {
				used[i] = true
			}
		}
	}
}

// quadCenter returns the mean of a symbol's corners.
func quadCenter(q [4]image.Point) (float64, float64) {
	var x, y float64
	for _, p := range q {
		x += float64(p.X)
		y += float64(p.Y)
	}
	return x / 4, y / 4
}

// inQuad reports whether (x, y) lies inside the convex quadrilateral q,
// whose corners may run either way round.
func inQuad(q [4]image.Point, x, y float64) bool {
	var pos, neg bool
	for i := range q {
		a, b := q[i], q[(i+1)%4]
		cross := float64(b.X-a.X)*(y-float64(a.Y)) - float64(b.Y-a.Y)*(x-float64(a.X))
		pos = pos || cross > 0
		neg = neg || cross < 0
	}
	return !(pos && neg)
}

// readingOrder sorts symbols into rows from top to bottom, each from left
// to right. A row starts with the topmost symbol left and takes every
// symbol whose center is within half that symbol's height of its center.
func readingOrder(rs []*DecodeResult) []*DecodeResult {
	type item struct {
		r      *DecodeResult
		cx, cy float64
		half   float64
	}
	items := make([]item, len(rs))
	for i, r := range rs {
		cx, cy := quadCenter(r.Corners)
		minY, maxY := math.Inf(1), math.Inf(-1)
		for _, p := range r.Corners {
			minY, maxY = math.Min(minY, float64(p.Y)), math.Max(maxY, float64(p.Y))
		}
		items[i] = item{r, cx, cy, (maxY - minY) / 2}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].cy != items[j].cy {
			return items[i].cy < items[j].cy
		}
		return items[i].cx < items[j].cx
	})
	out := make([]*DecodeResult, 0, len(rs))
	for len(items) > 0 {
		top := items[0]
		var row, rest []item
		for _, it := range items {
			if math.Abs(it.cy-top.cy) <= top.half {
				row = append(row, it)
			} else {
				rest = append(rest, it)
			}
		}
		sort.SliceStable(row, func(i, j int) bool { return row[i].cx < row[j].cx })
		for _, it := range row {
			out = append(out, it.r)
		}
		items = rest
	}
	return out
}
