package detect

import (
	"encoding/binary"
	"fmt"
	"image"
	"sort"

	"github.com/piglig/go-qr/v2/internal/errs"
	"github.com/piglig/go-qr/v2/internal/symbol"
)

// Luma is a luminance image, one byte per pixel.
type Luma struct {
	L    []uint8
	W, H int
}

// Pyramid holds an image and its halvings, made as they are needed.
type Pyramid []Luma

func NewPyramid(l []uint8, w, h int) *Pyramid { return &Pyramid{{l, w, h}} }

// Level returns the image halved k times.
func (p *Pyramid) Level(k int) Luma {
	for len(*p) <= k {
		last := (*p)[len(*p)-1]
		small, sw, sh := Halve(last.L, last.W, last.H)
		*p = append(*p, Luma{small, sw, sh})
	}
	return (*p)[k]
}

// SearchOrder returns the Pyramid levels the robust search locates symbols
// in for a w×h image, in order: from the level nearest SearchPixels to the
// coarsest whose shorter side has MinScaledSide pixels, then finer down to
// full resolution, level 0.
func SearchOrder(w, h int) []int {
	levels := 0
	for min(w, h)>>(levels+1) >= MinScaledSide {
		levels++
	}
	start := 0
	for start < levels && w*h>>(2*start) > SearchPixels {
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

// SearchPixels is the size, in pixels, of the Pyramid level the robust
// search starts at; FastFirstPixels the largest image the fast path is
// tried on before it.
const (
	SearchPixels    = 500_000
	FastFirstPixels = 1_000_000
)

// MinScaledSide is the shortest side, in pixels, of the coarsest image the
// symbol is located in.
const MinScaledSide = 150

// Halve returns l downsampled by two with a 2×2 box filter, each pixel the
// rounded mean of four.
//
// It reads the rows a word of eight pixels at a time: the even and the odd
// pixels of a word, and of the word below, are added in four 16-bit lanes,
// which hold sums of four pixels, 1020 at most, and the rounded means are
// packed back into bytes. This is exact, and twice as fast as adding one
// pixel at a time.
func Halve(l []uint8, w, h int) ([]uint8, int, int) {
	nw, nh := w/2, h/2
	out := make([]uint8, nw*nh)
	for y := 0; y < nh; y++ {
		r0, r1 := l[2*y*w:(2*y+1)*w], l[(2*y+1)*w:(2*y+2)*w]
		o := out[y*nw : (y+1)*nw]
		x := 0
		for ; x+8 <= nw; x += 8 {
			lo := halveWords(binary.LittleEndian.Uint64(r0[2*x:]), binary.LittleEndian.Uint64(r1[2*x:]))
			hi := halveWords(binary.LittleEndian.Uint64(r0[2*x+8:]), binary.LittleEndian.Uint64(r1[2*x+8:]))
			binary.LittleEndian.PutUint64(o[x:], lo|hi<<32)
		}
		for ; x < nw; x++ {
			o[x] = uint8((int(r0[2*x]) + int(r0[2*x+1]) + int(r1[2*x]) + int(r1[2*x+1]) + 2) / 4)
		}
	}
	return out, nw, nh
}

// halveWords returns, in its low four bytes, the rounded means of the 2×2
// blocks of pixels in a and b, words of eight pixels of two rows.
func halveWords(a, b uint64) uint64 {
	const lanes = 0x00FF00FF00FF00FF
	s := a&lanes + a>>8&lanes + b&lanes + b>>8&lanes + 0x0002000200020002
	v := s >> 2 & lanes
	v = (v | v>>8) & 0x0000FFFF0000FFFF
	return (v | v>>16) & 0xFFFFFFFF
}

// FastSample samples a crisp, axis-aligned symbol from a luminance image,
// where a pixel is dark if its luminance is at most threshold (or above it,
// when inverted). It trims the light quiet zone to the dark bounding box,
// whose extent equals the symbol because finder patterns occupy three
// corners, derives the module pitch from the top-left finder's 7-module edge
// run, and samples each module center.
func FastSample(l []uint8, w, h int, threshold uint8, inverted bool) ([][]bool, image.Rectangle, error) {
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
		return nil, image.Rectangle{}, fmt.Errorf("%w: no dark pixels", errs.ErrNotFound)
	}

	pitch, ok := FinderPitch(dark, minX, minY, maxX, maxY)
	if !ok {
		return nil, image.Rectangle{}, fmt.Errorf("%w: no finder pattern at top-left", errs.ErrNotFound)
	}

	boxW, boxH := maxX-minX+1, maxY-minY+1
	size := int(float64(boxW)/pitch + 0.5)
	if size < 21 || size > 4*symbol.MaxVersion+17 || (size-17)%4 != 0 {
		return nil, image.Rectangle{}, fmt.Errorf("%w: inferred size %d is not a QR size", errs.ErrNotFound, size)
	}
	if vsize := int(float64(boxH)/pitch + 0.5); vsize != size {
		return nil, image.Rectangle{}, fmt.Errorf("%w: non-square module grid (%d vs %d)", errs.ErrNotFound, size, vsize)
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

// ToLuma converts img to 8-bit luminance, compositing translucent pixels over
// white so that a transparent background reads as light.
func ToLuma(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// Gray images and the Luma plane of YCbCr ones, which JPEG decodes to,
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
				row[x] = LumaPremul(uint32(p[i]), uint32(p[i+1]), uint32(p[i+2]), uint32(p[i+3]))
			}
		}
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			p := im.Pix[im.PixOffset(b.Min.X, b.Min.Y+y):]
			row := out[y*w : (y+1)*w]
			for x := range row {
				i := 4 * x
				a := uint32(p[i+3])
				row[x] = LumaPremul(uint32(p[i])*a/255, uint32(p[i+1])*a/255, uint32(p[i+2])*a/255, a)
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
			lut[i] = LumaPremul(r>>8, g>>8, bl>>8, a>>8)
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
				out[y*w+x] = LumaPremul(r>>8, g>>8, bl>>8, a>>8)
			}
		}
	}
	return out
}

// MinContrast is the smallest luminance range in which a symbol could be
// distinguished; flatter images are rejected without searching.
const MinContrast = 16

// LumaPremul returns the luminance of an 8-bit alpha-premultiplied color
// composited over white, with the coefficients of color.GrayModel.
func LumaPremul(r, g, b, a uint32) uint8 {
	return uint8((19595*r+38470*g+7471*b+1<<15)>>16 + 255 - a)
}

// OtsuThreshold returns the luminance t that maximizes the between-class
// variance of the classes [0, t] and (t, 255], and the lowest and highest
// luminance seen. The histogram is built from every other pixel, which is
// plenty to place the threshold.
func OtsuThreshold(l []uint8) (t, lo, hi uint8) {
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

// HybridThresholds thresholds each 8x8 block against the average black
// point of the surrounding 5x5 blocks, after ZXing's HybridBinarizer. A
// block with little contrast, such as the inside of a large module, takes
// the midpoint of the smallest surrounding window, in powers of two, that
// has contrast, or failing that its neighbors' black point. Small images
// take the global threshold. Pixels are classified as they are read, which
// saves thresholding the many that the finder scan skips and nothing else
// reads.
func HybridThresholds(l []uint8, w, h int) *ThresholdMap {
	const (
		block    = ThresholdBlock
		minRange = 24 // below this a block is considered uniform
	)
	if w < 5*block || h < 5*block {
		t, _, _ := OtsuThreshold(l)
		return &ThresholdMap{l: l, W: w, H: h, bw: 1, bh: 1, t: []uint8{t}, global: true}
	}
	bw, bh := (w+block-1)/block, (h+block-1)/block
	origin := func(i, limit int) int { return min(i*block, limit-block) }

	// Block statistics first; a Pyramid of their ranges serves blocks that
	// have no contrast of their own.
	type stat struct{ lo, hi, avg uint8 }
	stats := make([]stat, bw*bh)
	for by := 0; by < bh; by++ {
		y0 := origin(by, h)
		for bx := 0; bx < bw; bx++ {
			lo, hi, sum := blockStats(l[y0*w+origin(bx, w):], w)
			stats[by*bw+bx] = stat{lo, hi, uint8(sum / (block * block))}
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
	return &ThresholdMap{l: l, W: w, H: h, bw: bw, bh: bh, t: thr}
}

// blockStats returns the lowest and highest luminance and the sum of the
// ThresholdBlock×ThresholdBlock pixels whose top left is l[0], in rows w
// pixels apart.
//
// A row of a block is one word of eight pixels. The minimum and maximum of
// each of its bytes are taken down the rows, all eight at once, and only
// then across them; a pixel at a time, every comparison would wait for the
// one before it. The even and odd pixels are summed in 16-bit lanes, each
// the sum of 16 pixels, 4080 at most.
func blockStats(l []uint8, w int) (lo, hi uint8, sum int) {
	const lanes = 0x00FF00FF00FF00FF
	mn, mx, acc := ^uint64(0), uint64(0), uint64(0)
	for y := 0; y < ThresholdBlock; y++ {
		a := binary.LittleEndian.Uint64(l[y*w:])
		acc += a&lanes + a>>8&lanes
		mn ^= (mn ^ a) & byteGE(mn, a)
		mx ^= (mx ^ a) & byteGE(a, mx)
	}
	n := laneMin(mn&lanes, mn>>8&lanes)
	n = laneMin(n, n>>32)
	n = laneMin(n, n>>16)
	x := laneMax(mx&lanes, mx>>8&lanes)
	x = laneMax(x, x>>32)
	x = laneMax(x, x>>16)
	return uint8(n), uint8(x), int(acc * 0x0001000100010001 >> 48)
}

// byteGE returns 0xFF in each byte in which x is at least y, and 0 in the
// others (Hacker's Delight, 2nd ed., §2-18). Where the bytes' high bits
// differ, the one with it set is larger; where they agree, (x|H)-(y&^H)
// subtracts their low seven bits with a borrow from bit 7 alone, which
// stays set exactly when x's are at least y's.
func byteGE(x, y uint64) uint64 {
	const high = 0x8080808080808080
	t := (x | high) - (y &^ high)
	return (x&^y | ^(x^y)&t) & high >> 7 * 0xFF
}

// laneGE returns 0xFF in each 16-bit lane in which x is at least y, and 0
// in the others, for lanes of values up to 255: x+256-y has bit 8 set
// exactly then, and borrows from no other lane.
func laneGE(x, y uint64) uint64 {
	return ((x | 0x0100010001000100) - y) >> 8 & 0x0001000100010001 * 0xFF
}

// laneMin and laneMax return the lane-wise minimum and maximum of x and y,
// 16-bit lanes of values up to 255.
func laneMin(x, y uint64) uint64 { return x ^ (x^y)&laneGE(x, y) }
func laneMax(x, y uint64) uint64 { return y ^ (x^y)&laneGE(x, y) }

// ThresholdBlock is the side, in pixels, of the blocks a ThresholdMap
// holds one threshold for.
const ThresholdBlock = 8

// ThresholdMap classifies pixels as dark against a threshold per block of
// ThresholdBlock×ThresholdBlock pixels, or one global threshold. The last
// block of a row or column is aligned to the image edge, so it overlaps its
// neighbor; pixels in the overlap take the last block's threshold.
type ThresholdMap struct {
	l      []uint8
	W, H   int
	bw, bh int
	t      []uint8
	global bool
}

// block returns the block row or column of pixel coordinate v in an extent
// of n pixels and nb blocks.
func (m *ThresholdMap) block(v, n, nb int) int {
	if m.global {
		return 0
	}
	if v >= n-ThresholdBlock {
		return nb - 1
	}
	return v / ThresholdBlock
}

// Dark reports whether pixel (x, y) is Dark, and false outside the image.
func (m *ThresholdMap) Dark(x, y int) bool {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	return m.l[y*m.W+x] <= m.t[m.block(y, m.H, m.bh)*m.bw+m.block(x, m.W, m.bw)]
}

// runs appends to dst the lengths of the runs of one class along row y,
// alternating light and dark and starting with light (of length zero when
// the row starts dark), and returns it.
func (m *ThresholdMap) runs(y int, inverted bool, dst []int) []int {
	src := m.l[y*m.W : (y+1)*m.W]
	t := m.t[m.block(y, m.H, m.bh)*m.bw:]
	prev, start := false, 0
	if m.global {
		for x, v := range src {
			if d := (v <= t[0]) != inverted; d != prev {
				dst = append(dst, x-start)
				prev, start = d, x
			}
		}
	} else {
		tail := m.W - ThresholdBlock
		for bx := 0; bx < m.bw; bx++ {
			x0, x1 := bx*ThresholdBlock, min((bx+1)*ThresholdBlock, tail)
			if bx == m.bw-1 {
				x0, x1 = tail, m.W
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
	return append(dst, m.W-start)
}

// FinderPitch measures the module pitch from the rows that cross the center
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
func FinderPitch(dark func(x, y int) bool, minX, minY, maxX, maxY int) (float64, bool) {
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
