package qr

import (
	"errors"
	"fmt"
	"image"
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

// DecodeResult is a decoded symbol.
type DecodeResult struct {
	Text     string // the payload as UTF-8
	Version  int
	ECC      ECC
	Mask     int
	Mirrored bool // the symbol was read from a mirror image
	Segments []DecodedSegment
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

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 21 || h < 21 {
		return nil, fmt.Errorf("%w: image too small (%dx%d)", ErrNotFound, w, h)
	}
	l := toLuma(img)

	var firstErr error
	try := func(grid [][]bool, err error) (*DecodeResult, error) {
		if err == nil {
			var res *DecodeResult
			if res, err = decodeGrid(grid); err == nil {
				return res, nil
			}
		}
		// Keep the most specific failure: a symbol that was found but not
		// readable says more than "not found".
		if firstErr == nil || errors.Is(firstErr, ErrNotFound) && !errors.Is(err, ErrNotFound) {
			firstErr = err
		}
		return nil, err
	}

	threshold := otsuThreshold(l)
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
		if res, err := try(robustSample(bm, w, h)); err == nil || errors.Is(err, ErrUnsupported) {
			return res, err
		}
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
	text, segs, err := parseBitstream(data, ver)
	if err != nil {
		return nil, err
	}
	return &DecodeResult{Text: text, Version: ver, ECC: ecc, Mask: mask, Segments: segs}, nil
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

	run := 0
	for x := minX; x <= maxX && dark(x, minY); x++ {
		run++
	}
	if run < 7 {
		return nil, fmt.Errorf("%w: no finder edge at top-left", ErrNotFound)
	}
	pitch := float64(run) / 7

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

// lumaPremul returns the luminance of an 8-bit alpha-premultiplied color
// composited over white, with the coefficients of color.GrayModel.
func lumaPremul(r, g, b, a uint32) uint8 {
	return uint8((19595*r+38470*g+7471*b+1<<15)>>16 + 255 - a)
}

// binarizeGlobal thresholds the whole image at the level that best separates
// its luminance histogram into two classes (Otsu's method).
func binarizeGlobal(l []uint8, w, h int) []bool {
	t := otsuThreshold(l)
	out := make([]bool, w*h)
	for i, v := range l {
		out[i] = v <= t
	}
	return out
}

// otsuThreshold returns the luminance t that maximizes the between-class
// variance of the classes [0, t] and (t, 255]. The histogram is built from
// every other pixel, which is plenty to place the threshold.
func otsuThreshold(l []uint8) uint8 {
	var hist [256]int
	for i := 0; i < len(l); i += 2 {
		hist[l[i]]++
	}
	total, sumAll := (len(l)+1)/2, 0
	for i, n := range hist {
		sumAll += i * n
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
	return uint8(bestT)
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
			t := sum / 25
			for y := y0; y < y0+block; y++ {
				for x := x0; x < x0+block; x++ {
					out[y*w+x] = int(l[y*w+x]) <= t
				}
			}
		}
	}
	return out
}
