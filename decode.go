package qr

import (
	"errors"
	"fmt"
	"image"
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

	// StructuredAppend is set when the symbol is part of a sequence.
	StructuredAppend *StructuredAppend
	// GS1 reports an FNC1-in-first-position symbol: Text is a GS1 element
	// string whose variable-length elements end with the GS character.
	GS1 bool

	codewords []byte // corrected data codewords, for Code.Verify
}

type decodeConfig struct {
	fastPathOnly bool
}

// DecodeOption configures Decode.
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
// path that locates the finder patterns with a locally adaptive threshold,
// which handles rotation, noise, uneven lighting and low contrast. Each path
// is tried on the image as is and inverted (light modules on a dark
// background), and each sampled symbol is also read mirrored. Perspective
// distortion, Micro QR and multiple symbols per image are not supported.
//
// The error wraps ErrNotFound when no symbol was located, ErrDecodeFailed
// when a symbol was located but could not be read, and ErrUnsupported for
// features this decoder does not implement.
func Decode(img image.Image, opts ...DecodeOption) (*DecodeResult, error) {
	var cfg decodeConfig
	for _, o := range opts {
		o(&cfg)
	}
	return searchImage(img, cfg, decodeGrid)
}

// searchImage runs the fast and robust samplers over img, each on the image
// as is and inverted, and returns the first grid that read decodes.
func searchImage(img image.Image, cfg decodeConfig, read func([][]bool) (*DecodeResult, error)) (*DecodeResult, error) {
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
	try := func(grid [][]bool, err error) (*DecodeResult, error) {
		if err == nil {
			var res *DecodeResult
			if res, err = read(grid); err == nil {
				return res, nil
			}
		}
		note(err)
		return nil, err
	}

	var adaptive []bool
	for _, inverted := range []bool{false, true} {
		if res, err := try(fastSample(l, w, h, threshold, inverted)); err == nil || errors.Is(err, ErrUnsupported) {
			return res, err
		}
		if cfg.fastPathOnly {
			continue
		}
		if adaptive == nil {
			adaptive = binarizeHybrid(l, w, h)
		}
		bm := adaptive
		if inverted {
			bm = invert(adaptive)
		}
		res, err := robustDecode(bm, w, h, read)
		if err == nil || errors.Is(err, ErrUnsupported) {
			return res, err
		}
		note(err)
	}
	return nil, firstErr
}

// decodeGrid decodes a sampled module grid, retrying it transposed (which is
// how a mirror image samples) if it cannot be read as is.
func decodeGrid(grid [][]bool) (*DecodeResult, error) {
	res, err := decodeModules(grid)
	if err == nil || errors.Is(err, ErrUnsupported) {
		return res, err
	}
	if res, err2 := decodeModules(transpose(grid)); err2 == nil || errors.Is(err2, ErrUnsupported) {
		if res != nil {
			res.Mirrored = true
		}
		return res, err2
	}
	return nil, err
}

func decodeModules(grid [][]bool) (*DecodeResult, error) {
	data, ver, ecc, mask, err := decodeMatrix(grid)
	if err != nil {
		return nil, err
	}
	bs, err := parseBitstream(data, ver)
	if err != nil {
		return nil, err
	}
	return &DecodeResult{
		Text: bs.text, Version: ver, ECC: ecc, Mask: mask, Segments: bs.segs,
		StructuredAppend: bs.structured, GS1: bs.gs1,
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

func invert(bm []bool) []bool {
	out := make([]bool, len(bm))
	for i, v := range bm {
		out[i] = !v
	}
	return out
}

// fastSample samples a crisp, axis-aligned symbol from a luminance image,
// where a pixel is dark if its luminance is at most threshold (or above it,
// when inverted). It trims the light quiet zone to the dark bounding box,
// whose extent equals the symbol because finder patterns occupy three
// corners, derives the module pitch from the top-left finder's 7-module edge
// run, and samples each module center.
func fastSample(l []uint8, w, h int, threshold uint8, inverted bool) ([][]bool, error) {
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
		return nil, fmt.Errorf("%w: no dark pixels", ErrNotFound)
	}

	pitch, ok := finderPitch(dark, minX, minY, maxX, maxY)
	if !ok {
		return nil, fmt.Errorf("%w: no finder pattern at top-left", ErrNotFound)
	}

	boxW, boxH := maxX-minX+1, maxY-minY+1
	size := int(float64(boxW)/pitch + 0.5)
	if size < 21 || size > 4*MaxVersion+17 || (size-17)%4 != 0 {
		return nil, fmt.Errorf("%w: inferred size %d is not a QR size", ErrNotFound, size)
	}
	if vsize := int(float64(boxH)/pitch + 0.5); vsize != size {
		return nil, fmt.Errorf("%w: non-square module grid (%d vs %d)", ErrNotFound, size, vsize)
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
	return modules, nil
}

// toLuma converts img to 8-bit luminance, compositing translucent pixels over
// white so that a transparent background reads as light.
func toLuma(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
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

// binarizeGlobal thresholds the whole image at the level that best separates
// its luminance histogram into two classes (Otsu's method).
func binarizeGlobal(l []uint8, w, h int) []bool {
	t, _, _ := otsuThreshold(l)
	out := make([]bool, w*h)
	for i, v := range l {
		out[i] = v <= t
	}
	return out
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

// binarizeHybrid thresholds each 8x8 block against the average black point
// of the surrounding 5x5 blocks, after ZXing's HybridBinarizer. Blocks with
// little contrast take their black point from their neighbors, so large
// uniform areas such as finder centers and quiet zones keep their class.
// Small images fall back to the global threshold.
func binarizeHybrid(l []uint8, w, h int) []bool {
	const (
		block    = 8
		minRange = 24 // below this a block is considered uniform
	)
	if w < 5*block || h < 5*block {
		return binarizeGlobal(l, w, h)
	}
	bw, bh := (w+block-1)/block, (h+block-1)/block
	origin := func(i, limit int) int { return min(i*block, limit-block) }

	black := make([]int, bw*bh)
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
			avg := sum / (block * block)
			if hi-lo <= minRange {
				// Uniform block: assume it is light unless the neighbors'
				// black point says otherwise.
				avg = lo / 2
				if by > 0 && bx > 0 {
					nb := (black[(by-1)*bw+bx] + 2*black[by*bw+bx-1] + black[(by-1)*bw+bx-1]) / 4
					if lo < nb {
						avg = nb
					}
				}
			}
			black[by*bw+bx] = avg
		}
	}

	out := make([]bool, w*h)
	clamp := func(v, n int) int { return max(2, min(v, n-3)) }
	for by := 0; by < bh; by++ {
		y0, cy := origin(by, h), clamp(by, bh)
		for bx := 0; bx < bw; bx++ {
			x0, cx := origin(bx, w), clamp(bx, bw)
			sum := 0
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					sum += black[(cy+dy)*bw+cx+dx]
				}
			}
			t := uint8(sum / 25)
			for y := y0; y < y0+block; y++ {
				dst := out[y*w+x0 : y*w+x0+block]
				for i, v := range l[y*w+x0 : y*w+x0+block] {
					dst[i] = v <= t
				}
			}
		}
	}
	return out
}

// JoinStructuredAppend reassembles the message of a structured append
// sequence from its decoded symbols, given in any order. Every symbol of the
// sequence must be present exactly once, and all must agree on the length
// and parity of the sequence; the error wraps ErrInvalidArgument otherwise.
func JoinStructuredAppend(parts ...*DecodeResult) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: no symbols to join", ErrInvalidArgument)
	}
	first := parts[0].StructuredAppend
	if first == nil {
		return "", fmt.Errorf("%w: symbol 0 is not part of a structured append sequence", ErrInvalidArgument)
	}
	ordered := make([]*DecodeResult, first.Total)
	for i, p := range parts {
		sa := p.StructuredAppend
		switch {
		case sa == nil:
			return "", fmt.Errorf("%w: symbol %d is not part of a structured append sequence", ErrInvalidArgument, i)
		case sa.Total != first.Total || sa.Parity != first.Parity:
			return "", fmt.Errorf("%w: symbol %d belongs to a different sequence", ErrInvalidArgument, i)
		case sa.Index >= sa.Total:
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
// ratio tolerance at about five times the real pitch.
func finderPitch(dark func(x, y int) bool, minX, minY, maxX, maxY int) (float64, bool) {
	maxTotal := (maxX-minX+1)/3 + 1
	var totals []int
	for y := minY; y <= minY+(maxY-minY)/2; y++ {
		if !dark(minX, y) {
			continue
		}
		var s [5]int
		state, x := 0, minX
		for ; x <= maxX && state < 5; x++ {
			if dark(x, y) != (state%2 == 0) {
				state++
				if state == 5 {
					break
				}
			}
			s[state]++
		}
		complete := state == 5 || (state == 4 && x > maxX)
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
