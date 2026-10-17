package qr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"sort"
	"strings"
)

// DecodedSegment describes one segment of a decoded symbol.
type DecodedSegment struct {
	Mode     Mode
	NumChars int // value of the character count indicator; 0 for ECI segments
	// ECI is the ECI assignment in effect for the segment (for an ECI segment,
	// the assignment it declares), or -1 if none.
	ECI int
	// Data is the raw payload: ASCII digits or characters for numeric and
	// alphanumeric segments, the bytes of a byte segment, and Shift_JIS for
	// Kanji. It is nil for ECI segments.
	Data []byte
}

// StructuredAppend identifies a symbol as one of a sequence that together
// carries a single message (ISO/IEC 18004 §8). See JoinStructuredAppend.
type StructuredAppend struct {
	Index  int  // position in the sequence, from 0
	Total  int  // number of symbols in the sequence, 1 to 16
	Parity byte // XOR of the message bytes, the same in every symbol
}

// DecodeResult is a decoded symbol.
type DecodeResult struct {
	Text     string // the payload as UTF-8
	Version  int
	ECC      ECC
	Mask     int
	Mirrored bool // the symbol was read from a mirror image
	Segments []DecodedSegment

	// Corners are the outer corners of the symbol in the image, quiet zone
	// excluded, in the symbol's own orientation: top-left, top-right,
	// bottom-right and bottom-left. They follow the symbol through rotation,
	// perspective and mirroring. Since v2.6.
	Corners [4]image.Point
	// Inverted reports a symbol of light modules on a dark background.
	// Since v2.6.
	Inverted bool

	// StructuredAppend is set when the symbol is part of a sequence.
	StructuredAppend *StructuredAppend
	// GS1 reports an FNC1-in-first-position symbol: Text is a GS1 element
	// string whose variable-length elements end with the GS character.
	GS1 bool

	// ErrorsCorrected is the number of codewords that error correction
	// repaired, over all blocks. Since v2.7.
	ErrorsCorrected int
	// UnusedErrorCorrection is the share, from 0 to 1, of the error
	// correction capacity that the block that needed most left unused, as
	// ISO/IEC 15415 defines it: 1 for a symbol read without errors, 0 for
	// one that needed all of it or more. Since v2.7.
	UnusedErrorCorrection float64
	// SoftDecision reports a symbol that hard decisions did not read, read
	// by soft decisions: with the modules decided again by a model of the
	// image and the least reliable codewords erased. Many readers decode
	// by hard decisions only. Since v2.7.
	SoftDecision bool

	codewords []byte         // corrected data codewords, for Code.Verify and Grade
	centers   *moduleCenters // filled in by locate when set, for Grade
}

type decodeConfig struct {
	fastPathOnly  bool
	maxSymbols    int
	maxSymbolsSet bool
	ctx           context.Context // nil for none
}

// canceled returns the error of c's context once it is done.
func (c *decodeConfig) canceled() error {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Err()
}

// DecodeOption configures Decode and DecodeAll.
type DecodeOption func(*decodeConfig)

// WithFastPathOnly restricts Decode to crisp, axis-aligned images such as the
// ones this package renders, skipping finder-pattern detection. Use it to
// verify generated codes quickly.
func WithFastPathOnly() DecodeOption {
	return func(c *decodeConfig) { c.fastPathOnly = true }
}

// Decode finds a QR Code in img and decodes it.
//
// It first tries a fast path for crisp, axis-aligned images, then a robust
// path that locates the finder patterns with a locally adaptive threshold
// and fits the module grid to the symbol's own structure, which handles
// rotation, perspective, lens distortion, noise, uneven lighting and low
// contrast; when a finder is lost, the other two imply it. Each path is
// tried on the image as is and inverted (light modules on a dark
// background), and each sampled symbol is also read mirrored. Micro QR and
// rMQR are not supported. Decode reads one symbol; DecodeAll reads every
// symbol in the image.
//
// The error wraps ErrNotFound when no symbol was located, ErrDecodeFailed
// when a symbol was located but could not be read, and ErrUnsupported for
// features this decoder does not implement.
func Decode(img image.Image, opts ...DecodeOption) (*DecodeResult, error) {
	return DecodeContext(context.Background(), img, opts...)
}

// DecodeContext is Decode with a context: when ctx is done before a symbol
// is read, it stops searching and returns ctx.Err(). It checks ctx between
// the stages of the search, such as the scales of the image, so it may go
// on for one stage after ctx is done; at full resolution a stage takes up
// to about half the time a large image without a symbol does. Use it to
// bound the time spent on images from untrusted sources, or on a frame of
// video once a newer one has arrived. Since v2.7.
func DecodeContext(ctx context.Context, img image.Image, opts ...DecodeOption) (*DecodeResult, error) {
	cfg := decodeConfig{ctx: ctx}
	for _, o := range opts {
		o(&cfg)
	}
	res, err := searchImage(img, cfg, decodeGrid)
	if res != nil {
		for i := range res.Corners {
			res.Corners[i] = res.Corners[i].Add(img.Bounds().Min)
		}
	}
	return res, err
}

// locate records where res was read from: the luminance image positions of
// the corners of the sampled module grid, which at maps from module space,
// and its polarity. A mirrored symbol was read transposed, so its own
// top-right corner is the grid's bottom-left.
func locate(res *DecodeResult, at func(u, v float64) (float64, float64), dim int, inverted bool) {
	if res == nil {
		return
	}
	d := float64(dim)
	corners := [4][2]float64{{0, 0}, {d, 0}, {d, d}, {0, d}}
	if res.Mirrored {
		corners[1], corners[3] = corners[3], corners[1]
	}
	for i, c := range corners {
		x, y := at(c[0], c[1])
		res.Corners[i] = image.Pt(int(math.Round(x)), int(math.Round(y)))
	}
	res.Inverted = inverted
	if res.centers != nil {
		res.centers.fill(at, dim)
	}
}

// searchImage runs the fast and robust samplers over img, each on the image
// as is and inverted, and returns the first grid that read decodes.
func searchImage(img image.Image, cfg decodeConfig, read gridReader) (*DecodeResult, error) {
	if err := cfg.canceled(); err != nil {
		return nil, err
	}
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
	// Keep the most specific failure: a symbol that was found but not
	// readable says more than "not found".
	note := func(err error) {
		if firstErr == nil || errors.Is(firstErr, ErrNotFound) && !errors.Is(err, ErrNotFound) {
			firstErr = err
		}
	}
	// try reads a grid from the fast path, which samples the box r.
	try := func(grid [][]bool, r image.Rectangle, inverted bool, err error) (*DecodeResult, error) {
		if err == nil {
			var res *DecodeResult
			if res, err = read(grid, nil); err == nil {
				d := float64(len(grid))
				locate(res, func(u, v float64) (float64, float64) {
					return float64(r.Min.X) + u/d*float64(r.Dx()), float64(r.Min.Y) + v/d*float64(r.Dy())
				}, len(grid), inverted)
				return res, nil
			}
		}
		note(err)
		return nil, err
	}

	// fast tries the fast path, which reads crisp, axis-aligned images such
	// as rendered ones.
	fast := func(inverted bool) (*DecodeResult, error, bool) {
		grid, box, err := fastSample(l, w, h, threshold, inverted)
		res, err := try(grid, box, inverted, err)
		return res, err, err == nil || errors.Is(err, ErrUnsupported)
	}
	if cfg.fastPathOnly {
		for _, inverted := range []bool{false, true} {
			if res, err, done := fast(inverted); done {
				return res, err
			}
		}
		return nil, firstErr
	}
	// Photos from cameras have far more pixels than their codes need, and
	// the fast path reads only rendered images, which are small.
	smallImage := w*h <= fastFirstPixels
	if smallImage {
		if res, err, done := fast(false); done {
			return res, err
		}
	}

	// The robust path locates symbols in a pyramid of the image, halved
	// level by level. A coarser level averages out texture finer than a
	// module, such as noise, a screen's pixel grid or halftone dots, and
	// fits very large modules into the local threshold's window; a finer
	// one resolves smaller modules. The modules are always read at full
	// resolution. Locating costs about the pixels of the level, so the
	// search starts at the level nearest searchPixels, where most symbols
	// in photos are found, goes coarser, and then finer down to full
	// resolution. It tries the same levels in any image as a search from
	// full resolution would, so it finds the same symbols, at a fraction
	// of the cost in camera photos.
	pyr := newPyramid(l, w, h)
	var full *thresholdMap // level 0's, also for the inverted image
	for _, k := range searchOrder(w, h) {
		if err := cfg.canceled(); err != nil {
			return nil, err
		}
		p := pyr.level(k)
		tm := hybridThresholds(p.l, p.w, p.h)
		if k == 0 {
			full = tm
		}
		// Regions are located again at full resolution only before it
		// has been searched as a whole.
		res, err := robustDecode(cfg.ctx, tm, false, 1<<k, full == nil, l, w, h, read)
		if err == nil || errors.Is(err, ErrUnsupported) {
			return res, err
		}
		note(err)
	}

	// Light symbols on a dark background, at full resolution.
	if err := cfg.canceled(); err != nil {
		return nil, err
	}
	if !smallImage {
		if res, err, done := fast(false); done {
			return res, err
		}
	}
	if res, err, done := fast(true); done {
		return res, err
	}
	if err := cfg.canceled(); err != nil {
		return nil, err
	}
	res, err := robustDecode(cfg.ctx, full, true, 1, false, l, w, h, read)
	if err == nil || errors.Is(err, ErrUnsupported) {
		return res, err
	}
	note(err)
	return nil, firstErr
}

// luma is a luminance image, one byte per pixel.
type luma struct {
	l    []uint8
	w, h int
}

// pyramid holds an image and its halvings, made as they are needed.
type pyramid []luma

func newPyramid(l []uint8, w, h int) *pyramid { return &pyramid{{l, w, h}} }

// level returns the image halved k times.
func (p *pyramid) level(k int) luma {
	for len(*p) <= k {
		last := (*p)[len(*p)-1]
		small, sw, sh := halve(last.l, last.w, last.h)
		*p = append(*p, luma{small, sw, sh})
	}
	return (*p)[k]
}

// searchOrder returns the pyramid levels the robust search locates symbols
// in for a w×h image, in order: from the level nearest searchPixels to the
// coarsest whose shorter side has minScaledSide pixels, then finer down to
// full resolution, level 0.
func searchOrder(w, h int) []int {
	levels := 0
	for min(w, h)>>(levels+1) >= minScaledSide {
		levels++
	}
	start := 0
	for start < levels && w*h>>(2*start) > searchPixels {
		start++
	}
	order := make([]int, 0, levels+1)
	for k := start; k <= levels; k++ {
		order = append(order, k)
	}
	for k := start - 1; k >= 0; k-- {
		order = append(order, k)
	}
	return order
}

// searchPixels is the size, in pixels, of the pyramid level the robust
// search starts at; fastFirstPixels the largest image the fast path is
// tried on before it.
const (
	searchPixels    = 500_000
	fastFirstPixels = 1_000_000
)

// minScaledSide is the shortest side, in pixels, of the coarsest image the
// symbol is located in.
const minScaledSide = 150

// halve returns l downsampled by two with a 2×2 box filter.
func halve(l []uint8, w, h int) ([]uint8, int, int) {
	nw, nh := w/2, h/2
	out := make([]uint8, nw*nh)
	for y := 0; y < nh; y++ {
		r0, r1 := l[2*y*w:], l[(2*y+1)*w:]
		for x := 0; x < nw; x++ {
			out[y*nw+x] = uint8((int(r0[2*x]) + int(r0[2*x+1]) + int(r1[2*x]) + int(r1[2*x+1]) + 2) / 4)
		}
	}
	return out, nw, nh
}

// decodeGrid decodes a sampled module grid, retrying it transposed (which is
// how a mirror image samples) if it cannot be read as is.
func decodeGrid(grid [][]bool, rel []float32) (*DecodeResult, error) {
	res, err := decodeModules(grid, rel)
	if err == nil || errors.Is(err, ErrUnsupported) {
		return res, err
	}
	if res, err2 := decodeModules(transpose(grid), transposeReliability(rel, len(grid))); err2 == nil || errors.Is(err2, ErrUnsupported) {
		if res != nil {
			res.Mirrored = true
		}
		return res, err2
	}
	return nil, err
}

func decodeModules(grid [][]bool, rel []float32) (*DecodeResult, error) {
	sd, err := decodeMatrix(grid, rel)
	if err != nil {
		return nil, err
	}
	bs, err := parseBitstream(sd.data, sd.ver)
	if err != nil {
		return nil, err
	}
	return &DecodeResult{
		Text: bs.text, Version: sd.ver, ECC: sd.ecc, Mask: sd.mask, Segments: bs.segs,
		StructuredAppend: bs.structured, GS1: bs.gs1,
		ErrorsCorrected: sd.corrected, UnusedErrorCorrection: sd.unusedEC, SoftDecision: rel != nil,
		codewords: sd.data,
	}, nil
}

func transpose(grid [][]bool) [][]bool {
	n := len(grid)
	out := make([][]bool, n)
	backing := make([]bool, n*n)
	for y := range out {
		out[y] = backing[y*n : (y+1)*n]
		for x := range out[y] {
			out[y][x] = grid[x][y]
		}
	}
	return out
}

// fastSample samples a crisp, axis-aligned symbol from a luminance image,
// where a pixel is dark if its luminance is at most threshold (or above it,
// when inverted). It trims the light quiet zone to the dark bounding box,
// whose extent equals the symbol because finder patterns occupy three
// corners, derives the module pitch from the top-left finder's 7-module edge
// run, and samples each module center.
func fastSample(l []uint8, w, h int, threshold uint8, inverted bool) ([][]bool, image.Rectangle, error) {
	dark := func(x, y int) bool { return (l[y*w+x] <= threshold) != inverted }
	// Scan each row inward from both ends, so mostly quiet zone is visited.
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		row := l[y*w : (y+1)*w]
		first := 0
		for first < w && (row[first] <= threshold) == inverted {
			first++
		}
		if first == w {
			continue
		}
		last := w - 1
		for (row[last] <= threshold) == inverted {
			last--
		}
		minX, maxX = min(minX, first), max(maxX, last)
		minY, maxY = min(minY, y), y
	}
	if maxX < 0 {
		return nil, image.Rectangle{}, fmt.Errorf("%w: no dark pixels", ErrNotFound)
	}

	pitch, ok := finderPitch(dark, minX, minY, maxX, maxY)
	if !ok {
		return nil, image.Rectangle{}, fmt.Errorf("%w: no finder pattern at top-left", ErrNotFound)
	}

	boxW, boxH := maxX-minX+1, maxY-minY+1
	size := int(float64(boxW)/pitch + 0.5)
	if size < 21 || size > 4*MaxVersion+17 || (size-17)%4 != 0 {
		return nil, image.Rectangle{}, fmt.Errorf("%w: inferred size %d is not a QR size", ErrNotFound, size)
	}
	if vsize := int(float64(boxH)/pitch + 0.5); vsize != size {
		return nil, image.Rectangle{}, fmt.Errorf("%w: non-square module grid (%d vs %d)", ErrNotFound, size, vsize)
	}

	px := float64(boxW) / float64(size)
	py := float64(boxH) / float64(size)
	modules := make([][]bool, size)
	grid := make([]bool, size*size)
	for row := 0; row < size; row++ {
		modules[row] = grid[row*size : (row+1)*size]
		cy := minY + int((float64(row)+0.5)*py)
		for col := 0; col < size; col++ {
			modules[row][col] = dark(minX+int((float64(col)+0.5)*px), cy)
		}
	}
	return modules, image.Rect(minX, minY, maxX+1, maxY+1), nil
}

// toLuma converts img to 8-bit luminance, compositing translucent pixels over
// white so that a transparent background reads as light.
func toLuma(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// Gray images and the luma plane of YCbCr ones, which JPEG decodes to,
	// are used in place when their rows are contiguous: the decoder only
	// reads the luminance.
	switch im := img.(type) {
	case *image.Gray:
		if im.Stride == w {
			off := im.PixOffset(b.Min.X, b.Min.Y)
			return im.Pix[off : off+w*h : off+w*h]
		}
	case *image.YCbCr:
		if im.YStride == w {
			off := im.YOffset(b.Min.X, b.Min.Y)
			return im.Y[off : off+w*h : off+w*h]
		}
	}
	out := make([]uint8, w*h)
	switch im := img.(type) {
	case *image.Gray:
		for y := 0; y < h; y++ {
			copy(out[y*w:(y+1)*w], im.Pix[im.PixOffset(b.Min.X, b.Min.Y+y):])
		}
	case *image.YCbCr:
		for y := 0; y < h; y++ {
			copy(out[y*w:(y+1)*w], im.Y[im.YOffset(b.Min.X, b.Min.Y+y):])
		}
	case *image.RGBA:
		for y := 0; y < h; y++ {
			p := im.Pix[im.PixOffset(b.Min.X, b.Min.Y+y):]
			row := out[y*w : (y+1)*w]
			for x := range row {
				i := 4 * x
				row[x] = lumaPremul(uint32(p[i]), uint32(p[i+1]), uint32(p[i+2]), uint32(p[i+3]))
			}
		}
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			p := im.Pix[im.PixOffset(b.Min.X, b.Min.Y+y):]
			row := out[y*w : (y+1)*w]
			for x := range row {
				i := 4 * x
				a := uint32(p[i+3])
				row[x] = lumaPremul(uint32(p[i])*a/255, uint32(p[i+1])*a/255, uint32(p[i+2])*a/255, a)
			}
		}
	case *image.Paletted:
		// Convert each palette entry once; PNGs written by this package are
		// two-color paletted images.
		var lut [256]uint8
		for i := range lut {
			lut[i] = 255 // out-of-palette indices read as light
		}
		for i, c := range im.Palette {
			r, g, bl, a := c.RGBA()
			lut[i] = lumaPremul(r>>8, g>>8, bl>>8, a>>8)
		}
		for y := 0; y < h; y++ {
			p := im.Pix[im.PixOffset(b.Min.X, b.Min.Y+y):]
			for x := 0; x < w; x++ {
				out[y*w+x] = lut[p[x]]
			}
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				out[y*w+x] = lumaPremul(r>>8, g>>8, bl>>8, a>>8)
			}
		}
	}
	return out
}

// minContrast is the smallest luminance range in which a symbol could be
// distinguished; flatter images are rejected without searching.
const minContrast = 16

// lumaPremul returns the luminance of an 8-bit alpha-premultiplied color
// composited over white, with the coefficients of color.GrayModel.
func lumaPremul(r, g, b, a uint32) uint8 {
	return uint8((19595*r+38470*g+7471*b+1<<15)>>16 + 255 - a)
}

// otsuThreshold returns the luminance t that maximizes the between-class
// variance of the classes [0, t] and (t, 255], and the lowest and highest
// luminance seen. The histogram is built from every other pixel, which is
// plenty to place the threshold.
func otsuThreshold(l []uint8) (t, lo, hi uint8) {
	var hist [256]int
	for i := 0; i < len(l); i += 2 {
		hist[l[i]]++
	}
	total, sumAll := (len(l)+1)/2, 0
	for i, n := range hist {
		sumAll += i * n
	}
	for lo < 255 && hist[lo] == 0 {
		lo++
	}
	for hi = 255; hi > 0 && hist[hi] == 0; hi-- {
	}

	var best float64
	bestT, wB, sumB := 127, 0, 0
	for t, n := range hist {
		wB += n
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += t * n
		mB := float64(sumB) / float64(wB)
		mF := float64(sumAll-sumB) / float64(wF)
		if v := float64(wB) * float64(wF) * (mB - mF) * (mB - mF); v > best {
			best, bestT = v, t
		}
	}
	return uint8(bestT), lo, hi
}

// hybridThresholds thresholds each 8x8 block against the average black
// point of the surrounding 5x5 blocks, after ZXing's HybridBinarizer. A
// block with little contrast, such as the inside of a large module, takes
// the midpoint of the smallest surrounding window, in powers of two, that
// has contrast, or failing that its neighbors' black point. Small images
// take the global threshold. Pixels are classified as they are read, which
// saves thresholding the many that the finder scan skips and nothing else
// reads.
func hybridThresholds(l []uint8, w, h int) *thresholdMap {
	const (
		block    = thresholdBlock
		minRange = 24 // below this a block is considered uniform
	)
	if w < 5*block || h < 5*block {
		t, _, _ := otsuThreshold(l)
		return &thresholdMap{l: l, w: w, h: h, bw: 1, bh: 1, t: []uint8{t}, global: true}
	}
	bw, bh := (w+block-1)/block, (h+block-1)/block
	origin := func(i, limit int) int { return min(i*block, limit-block) }

	// Block statistics first; a pyramid of their ranges serves blocks that
	// have no contrast of their own.
	type stat struct{ lo, hi, avg uint8 }
	stats := make([]stat, bw*bh)
	for by := 0; by < bh; by++ {
		y0 := origin(by, h)
		for bx := 0; bx < bw; bx++ {
			x0 := origin(bx, w)
			lo, hi, sum := 255, 0, 0
			for y := y0; y < y0+block; y++ {
				for _, v := range l[y*w+x0 : y*w+x0+block] {
					sum += int(v)
					lo, hi = min(lo, int(v)), max(hi, int(v))
				}
			}
			stats[by*bw+bx] = stat{uint8(lo), uint8(hi), uint8(sum / (block * block))}
		}
	}
	pyr := newRangePyramid(bw, bh, func(i int) (uint8, uint8) { return stats[i].lo, stats[i].hi })

	black := make([]uint8, bw*bh)
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			st := stats[by*bw+bx]
			avg := int(st.avg)
			if int(st.hi)-int(st.lo) <= minRange {
				// Uniform block. It may lie inside a module larger than the
				// thresholding window: take the midpoint of the smallest
				// surrounding window, in powers of two, that has contrast.
				if lo, hi, ok := pyr.contrast(bx, by, minRange); ok {
					avg = (lo + hi) / 2
				} else {
					// No contrast anywhere near: assume it is light unless the
					// neighbors' black point says otherwise.
					avg = int(st.lo) / 2
					if by > 0 && bx > 0 {
						nb := (int(black[(by-1)*bw+bx]) + 2*int(black[by*bw+bx-1]) + int(black[(by-1)*bw+bx-1])) / 4
						if int(st.lo) < nb {
							avg = nb
						}
					}
				}
			}
			black[by*bw+bx] = uint8(avg)
		}
	}

	thr := make([]uint8, bw*bh)
	clamp := func(v, n int) int { return max(2, min(v, n-3)) }
	for by := 0; by < bh; by++ {
		cy := clamp(by, bh)
		for bx := 0; bx < bw; bx++ {
			cx := clamp(bx, bw)
			sum := 0
			for dy := -2; dy <= 2; dy++ {
				for _, b := range black[(cy+dy)*bw+cx-2 : (cy+dy)*bw+cx+3] {
					sum += int(b)
				}
			}
			thr[by*bw+bx] = uint8(sum / 25)
		}
	}
	return &thresholdMap{l: l, w: w, h: h, bw: bw, bh: bh, t: thr}
}

// thresholdBlock is the side, in pixels, of the blocks a thresholdMap
// holds one threshold for.
const thresholdBlock = 8

// thresholdMap classifies pixels as dark against a threshold per block of
// thresholdBlock×thresholdBlock pixels, or one global threshold. The last
// block of a row or column is aligned to the image edge, so it overlaps its
// neighbor; pixels in the overlap take the last block's threshold.
type thresholdMap struct {
	l      []uint8
	w, h   int
	bw, bh int
	t      []uint8
	global bool
}

// block returns the block row or column of pixel coordinate v in an extent
// of n pixels and nb blocks.
func (m *thresholdMap) block(v, n, nb int) int {
	if m.global {
		return 0
	}
	if v >= n-thresholdBlock {
		return nb - 1
	}
	return v / thresholdBlock
}

// dark reports whether pixel (x, y) is dark, and false outside the image.
func (m *thresholdMap) dark(x, y int) bool {
	if x < 0 || y < 0 || x >= m.w || y >= m.h {
		return false
	}
	return m.l[y*m.w+x] <= m.t[m.block(y, m.h, m.bh)*m.bw+m.block(x, m.w, m.bw)]
}

// runs appends to dst the lengths of the runs of one class along row y,
// alternating light and dark and starting with light (of length zero when
// the row starts dark), and returns it.
func (m *thresholdMap) runs(y int, inverted bool, dst []int) []int {
	src := m.l[y*m.w : (y+1)*m.w]
	t := m.t[m.block(y, m.h, m.bh)*m.bw:]
	prev, start := false, 0
	if m.global {
		for x, v := range src {
			if d := (v <= t[0]) != inverted; d != prev {
				dst = append(dst, x-start)
				prev, start = d, x
			}
		}
	} else {
		tail := m.w - thresholdBlock
		for bx := 0; bx < m.bw; bx++ {
			x0, x1 := bx*thresholdBlock, min((bx+1)*thresholdBlock, tail)
			if bx == m.bw-1 {
				x0, x1 = tail, m.w
			}
			tb := t[bx]
			for x := x0; x < x1; x++ {
				if d := (src[x] <= tb) != inverted; d != prev {
					dst = append(dst, x-start)
					prev, start = d, x
				}
			}
		}
	}
	return append(dst, m.w-start)
}

// JoinStructuredAppend reassembles the message of a structured append
// sequence from its decoded symbols, given in any order. Every symbol of the
// sequence must be present exactly once, and all must agree on the length
// and parity of the sequence; the error wraps ErrInvalidArgument otherwise,
// and for a nil part.
func JoinStructuredAppend(parts ...*DecodeResult) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: no symbols to join", ErrInvalidArgument)
	}
	var first *StructuredAppend
	if parts[0] != nil {
		first = parts[0].StructuredAppend
	}
	switch {
	case first == nil:
		return "", fmt.Errorf("%w: symbol 0 is not part of a structured append sequence", ErrInvalidArgument)
	case first.Total < 1 || first.Total > maxStructuredAppend:
		return "", fmt.Errorf("%w: sequence of %d symbols", ErrInvalidArgument, first.Total)
	}
	ordered := make([]*DecodeResult, first.Total)
	for i, p := range parts {
		if p == nil {
			return "", fmt.Errorf("%w: symbol %d is nil", ErrInvalidArgument, i)
		}
		sa := p.StructuredAppend
		switch {
		case sa == nil:
			return "", fmt.Errorf("%w: symbol %d is not part of a structured append sequence", ErrInvalidArgument, i)
		case sa.Total != first.Total || sa.Parity != first.Parity:
			return "", fmt.Errorf("%w: symbol %d belongs to a different sequence", ErrInvalidArgument, i)
		case sa.Index < 0 || sa.Index >= sa.Total:
			return "", fmt.Errorf("%w: symbol %d has index %d of %d", ErrInvalidArgument, i, sa.Index, sa.Total)
		case ordered[sa.Index] != nil:
			return "", fmt.Errorf("%w: duplicate symbol %d of the sequence", ErrInvalidArgument, sa.Index)
		}
		ordered[sa.Index] = p
	}
	var sb strings.Builder
	for i, p := range ordered {
		if p == nil {
			return "", fmt.Errorf("%w: symbol %d of %d is missing", ErrInvalidArgument, i, first.Total)
		}
		sb.WriteString(p.Text)
	}
	return sb.String(), nil
}

// finderPitch measures the module pitch from the rows that cross the center
// of the top-left finder pattern, where the runs from the left edge of the
// symbol read 1:1:3:1:1. Using the center rows rather than the top edge
// keeps this exact for rounded and circular finder styles.
//
// The finder spans 7 of at least 21 modules, so a run sequence wider than a
// third of the symbol is not the finder, even if its ratios match: the top
// row 7:5:9:5:7 (the finder edge, then data) fits 1:1:3:1:1 within the
// ratio tolerance at about five times the real pitch. For the same reason
// only the top third of the rows can cross the finder, and each row is
// followed no further than a third of the width.
func finderPitch(dark func(x, y int) bool, minX, minY, maxX, maxY int) (float64, bool) {
	maxTotal := (maxX-minX+1)/3 + 1
	maxY = min(maxY, minY+(maxY-minY+1)/3+1)
	stopX := min(maxX, minX+maxTotal)
	var totals []int
	for y := minY; y <= maxY; y++ {
		if !dark(minX, y) {
			continue
		}
		var s [5]int
		state, x := 0, minX
		for ; x <= stopX && state < 5; x++ {
			if dark(x, y) != (state%2 == 0) {
				state++
				if state == 5 {
					break
				}
			}
			s[state]++
		}
		complete := state == 5 || (state == 4 && x > stopX && stopX == maxX)
		total := s[0] + s[1] + s[2] + s[3] + s[4]
		if _, ok := checkFinderRatio(s); complete && ok && total <= maxTotal {
			totals = append(totals, total)
		} else if len(totals) > 0 {
			break // past the finder's center rows
		}
	}
	if len(totals) == 0 {
		return 0, false
	}
	sort.Ints(totals)
	return float64(totals[len(totals)/2]) / 7, true
}
