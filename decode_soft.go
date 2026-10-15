package qr

import (
	"errors"
	"math"
	"sort"

	"github.com/piglig/go-qr/v2/internal/reedsolomon"
)

// Soft-decision decoding. A grid is first read as it always was: each module
// dark when darker than the modules around it, and each block corrected by
// hard decisions. When that fails on a grid whose known modules (the function
// patterns) read back as a symbol's do, the grid most likely sits on a symbol
// whose modules glare, blur or damage made hard to tell apart. The decoder
// then uses how sure it is of each module: the least reliable codewords of a
// block are erased, which Reed–Solomon codes correct twice as many of as
// errors (generalized minimum distance decoding, Forney 1966); and the
// modules are classified again by a model of the image fitted to the known
// modules (classifyModules).
//
// Attempts beyond the first hard decision raise the chance of correcting
// noise into a wrong codeword, so they keep redundancy unused: the
// misdecode protection codewords of ISO/IEC 18004 Table 9, which hard
// decoding uses as other decoders do, and gmdMargin bytes more. They must
// also read at least one data segment. In 3 million random grids whose
// function patterns were drawn intact, none decoded.

// gridReader decodes a sampled module grid. rel, if not nil, holds the
// reliability of each module, row by row, for an attempt beyond the first
// hard decision.
type gridReader func(grid [][]bool, rel []float32) (*DecodeResult, error)

// softGate is the fraction of the known modules a grid must read right for
// the attempts beyond the first hard decision: grids of photos that soft
// decoding reads score 77% and more, false grids of texture far less.
const softGate = 0.75

// gmdMargin is the redundancy, in bytes per block, that erasure decoding
// leaves unused beyond the misdecode protection codewords. Using all of it,
// erasing every check byte would always yield some codeword.
const gmdMargin = 2

// readSoft reads a grid from its modules' luminances, row by row, and
// decodes it with read: by hard decisions first, then, if the grid passes
// softGate, with erasures and with the modules classified again.
func readSoft(vals []float64, dim int, inverted bool, read gridReader) (*DecodeResult, error) {
	grid := thresholdModules(vals, dim, inverted)
	res, err := read(grid, nil)
	if err == nil || errors.Is(err, ErrUnsupported) || pilotAccuracy(grid) < softGate {
		return res, err
	}
	if r, e := read(grid, thresholdReliability(vals, dim)); e == nil && hasData(r) || errors.Is(e, ErrUnsupported) {
		return r, e
	}
	if eq, rel := classifyModules(vals, dim, inverted); eq != nil {
		if r, e := read(eq, rel); e == nil && hasData(r) || errors.Is(e, ErrUnsupported) {
			return r, e
		}
	}
	return res, err
}

// thresholdReliability returns how far each module's luminance lies from
// the mean it is thresholded at, relative to the mean distance over the
// grid.
func thresholdReliability(vals []float64, dim int) []float32 {
	w := newModuleWindows(vals, dim)
	rel := make([]float32, dim*dim)
	sum := 0.0
	for r := 0; r < dim; r++ {
		for c := 0; c < dim; c++ {
			d := math.Abs(vals[r*dim+c] - w.mean(r, c))
			rel[r*dim+c] = float32(d)
			sum += d
		}
	}
	if sum > 0 {
		scale := float32(float64(len(rel)) / sum)
		for i := range rel {
			rel[i] *= scale
		}
	}
	return rel
}

// pilotAccuracy returns the fraction of the known modules of grid that read
// as they must. The function patterns are their own transposes, so it holds
// for a mirror image.
func pilotAccuracy(grid [][]bool) float64 {
	dim := len(grid)
	ver := (dim - 17) / 4
	if ver < MinVersion || ver > MaxVersion || 4*ver+17 != dim {
		return 0
	}
	t := getTemplate(ver)
	ok, total := 0, 0
	for i, known := range t.pilot {
		if known {
			total++
			if grid[i/dim][i%dim] == t.pilotDark[i] {
				ok++
			}
		}
	}
	return float64(ok) / float64(total)
}

// isFormatModule reports whether module (x, y) of a symbol n modules wide
// holds format information, or the dark module beside it.
func isFormatModule(x, y, n int) bool {
	return y == 8 && (x <= 8 || x >= n-8) || x == 8 && (y <= 8 || y >= n-8)
}

// hasData reports whether res holds a data segment with characters.
func hasData(res *DecodeResult) bool {
	if res == nil {
		return false
	}
	for _, seg := range res.Segments {
		switch seg.Mode {
		case ModeNumeric, ModeAlphanumeric, ModeByte, ModeKanji:
			if seg.NumChars > 0 {
				return true
			}
		}
	}
	return false
}

// transposeReliability returns rel, the reliabilities of a dim×dim grid row
// by row, for the transposed grid.
func transposeReliability(rel []float32, dim int) []float32 {
	if rel == nil {
		return nil
	}
	t := make([]float32, len(rel))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			t[y*dim+x] = rel[x*dim+y]
		}
	}
	return t
}

// codewordReliability returns the reliability of each raw codeword of a
// symbol of the given version, in placement order: that of its least
// reliable module.
func codewordReliability(rel []float32, ver int) []float32 {
	n := 4*ver + 17
	out := make([]float32, numRawDataModules(ver)/8)
	for i := range out {
		out[i] = float32(math.Inf(1))
	}
	bitAt := getTemplate(ver).bitAt
	for y := 0; y < n; y++ {
		for x, b := range bitAt[y] {
			if b >= 0 && int(b)/8 < len(out) {
				out[b/8] = min(out[b/8], rel[y*n+x])
			}
		}
	}
	return out
}

// softCorrect corrects a block of an attempt beyond the first hard decision:
// by hard decision within budget, then by erasing its 2, 4, ... least
// reliable bytes, rel[i] being byte i's reliability.
func softCorrect(codeword []byte, eccLen int, rel []float32, budget int) error {
	err := reedsolomon.CorrectErasures(codeword, eccLen, nil, budget)
	if err == nil || budget < 2 {
		return err
	}
	order := make([]int, len(codeword))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return rel[order[a]] < rel[order[b]] })
	try := make([]byte, len(codeword))
	for s := 2; s <= budget; s += 2 {
		copy(try, codeword)
		if reedsolomon.CorrectErasures(try, eccLen, order[:s], budget) == nil {
			copy(codeword, try)
			return nil
		}
	}
	return err
}
