package qr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/piglig/go-qr/v2/internal/detect"
	"github.com/piglig/go-qr/v2/internal/symbol"
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
	// detect.Perspective and mirroring. Since v2.6.
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
	noInverted    bool
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

// WithoutInverted skips the search for light symbols on a dark background,
// which Decode and DecodeAll make last, once the image holds no other
// symbol they can read. It saves the most on images without a symbol,
// such as most frames of a video in which a code is being looked for.
// Since v2.7.
func WithoutInverted() DecodeOption {
	return func(c *decodeConfig) { c.noInverted = true }
}

// polarities returns whether to search for symbols as is and inverted.
func (c *decodeConfig) polarities() []bool {
	if c.noInverted {
		return bothPolarities[:1]
	}
	return bothPolarities[:]
}

var bothPolarities = [2]bool{false, true}

// Decode finds a QR Code in img and decodes it.
//
// It first tries a fast path for crisp, axis-aligned images, then a robust
// path that locates the finder patterns with a locally adaptive threshold
// and fits the module grid to the symbol's own structure, which handles
// rotation, detect.Perspective, lens distortion, noise, uneven lighting and low
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
	l := detect.ToLuma(img)
	threshold, lo, hi := detect.OtsuThreshold(l)
	if hi-lo < detect.MinContrast {
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
		grid, box, err := detect.FastSample(l, w, h, threshold, inverted)
		res, err := try(grid, box, inverted, err)
		return res, err, err == nil || errors.Is(err, ErrUnsupported)
	}
	if cfg.fastPathOnly {
		for _, inverted := range cfg.polarities() {
			if res, err, done := fast(inverted); done {
				return res, err
			}
		}
		return nil, firstErr
	}
	// Photos from cameras have far more pixels than their codes need, and
	// the fast path reads only rendered images, which are small.
	smallImage := w*h <= detect.FastFirstPixels
	if smallImage {
		if res, err, done := fast(false); done {
			return res, err
		}
	}

	// The robust path locates symbols in a detect.Pyramid of the image, halved
	// level by level. A coarser level averages out texture finer than a
	// module, such as noise, a screen's pixel grid or halftone dots, and
	// fits very large modules into the local threshold's window; a finer
	// one resolves smaller modules. The modules are always read at full
	// resolution. Locating costs about the pixels of the level, so the
	// search starts at the level nearest detect.SearchPixels, where most symbols
	// in photos are found, goes coarser, and then finer down to full
	// resolution. It tries the same levels in any image as a search from
	// full resolution would, so it finds the same symbols, at a fraction
	// of the cost in camera photos.
	pyr := detect.NewPyramid(l, w, h)
	var full *detect.ThresholdMap // level 0's, also for the inverted image
	for _, k := range detect.SearchOrder(w, h) {
		if err := cfg.canceled(); err != nil {
			return nil, err
		}
		p := pyr.Level(k)
		tm := detect.HybridThresholds(p.L, p.W, p.H)
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

	if err := cfg.canceled(); err != nil {
		return nil, err
	}
	if !smallImage {
		if res, err, done := fast(false); done {
			return res, err
		}
	}
	// Light symbols on a dark background, at full resolution.
	if cfg.noInverted {
		return nil, firstErr
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
	sd, err := symbol.ReadMatrix(grid, rel)
	if err != nil {
		return nil, err
	}
	bs, err := parseBitstream(sd.Codewords, sd.Version)
	if err != nil {
		return nil, err
	}
	return &DecodeResult{
		Text: bs.text, Version: sd.Version, ECC: ECC(sd.ECC), Mask: sd.Mask, Segments: bs.segs,
		StructuredAppend: bs.structured, GS1: bs.gs1,
		ErrorsCorrected: sd.Corrected, UnusedErrorCorrection: sd.UnusedEC, SoftDecision: rel != nil,
		codewords: sd.Codewords,
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
