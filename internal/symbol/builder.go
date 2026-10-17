package symbol

import (
	"fmt"
	"math"

	"github.com/piglig/go-qr/v2/internal/errs"
	"github.com/piglig/go-qr/v2/internal/reedsolomon"
)

// Builder is the mutable scaffold used to lay out a QR matrix. It owns the
// module grid plus the function-module map (IsFunction) that drives codeword
// placement and masking. The encoder drives a Builder to completion and then
// hands its modules to an immutable qr.Code; the decoder reuses a Builder
// purely to rebuild the function map and undo masking. Keeping this state off
// the Code means the code is immutable and carries no build-time scratch.
type Builder struct {
	Version    int
	Size       int
	ECC        Level
	Modules    [][]bool // dark/light state of every module
	IsFunction [][]bool // true where a module belongs to a function pattern

	// packed holds four lines per row for mask selection: the modules by
	// row, which drawCodewords packs, then by column, then the same for the
	// masked scratch.
	packed []moduleLine
}

// NewBuilder allocates a blank Builder for the given version and ECC level.
func NewBuilder(version int, ecl Level) *Builder {
	size := version*4 + 17
	b := &Builder{Version: version, Size: size, ECC: ecl}
	// One backing allocation per grid; rows alias into it.
	b.Modules = make([][]bool, size)
	b.IsFunction = make([][]bool, size)
	modBacking := make([]bool, size*size)
	fnBacking := make([]bool, size*size)
	for i := 0; i < size; i++ {
		b.Modules[i] = modBacking[i*size : (i+1)*size]
		b.IsFunction[i] = fnBacking[i*size : (i+1)*size]
	}
	return b
}

// ChooseBestMask scores all eight mask patterns and returns the one with the
// lowest penalty. It does not mutate the module grid (masking is scored
// out-of-place); the caller applies the winning mask afterwards.
func (q *Builder) ChooseBestMask() int {
	minPenalty := math.MaxInt32
	best := 0
	n, lines := q.Size, q.packed
	modules := packedModules{size: n, rows: lines[:n:n], cols: lines[n : 2*n : 2*n]}
	masked := packedModules{size: n, rows: lines[2*n : 3*n : 3*n], cols: lines[3*n:]}
	modules.transposeRows()
	// Version-invariant mask patterns, built once per version and reused across
	// all encodes of that version (e.g. a batch of similar payloads).
	tmpl := getTemplate(q.Version)
	for i := 0; i < 8; i++ {
		q.DrawFormatBits(i)
		// The format bits all lie in row 8 and column 8.
		for k := 0; k < q.Size; k++ {
			modules.set(k, 8, q.Modules[8][k])
			modules.set(8, k, q.Modules[k][8])
		}
		masked.xor(&modules, &tmpl.maskPatterns[i])
		penalty := masked.penalty(minPenalty)
		if penalty < minPenalty {
			best = i
			minPenalty = penalty
		}
	}
	return best
}

// setFunctionModule paints a module and marks it as part of a function pattern.
func (q *Builder) setFunctionModule(x, y int, isDark bool) {
	q.Modules[y][x] = isDark
	q.IsFunction[y][x] = true
}

// addEccAndInterLeave appends Reed-Solomon ECC bytes to the raw data and
// interleaves the result according to the block layout for the QR version and
// ECC level.
func (q *Builder) AddECCAndInterleave(data []byte) ([]byte, error) {
	NumDataCodewords := NumDataCodewords(q.Version, q.ECC)
	if len(data) != NumDataCodewords {
		return nil, fmt.Errorf("%w: data length %d != expected %d", errs.ErrInvalidArgument, len(data), NumDataCodewords)
	}

	numBlocks := NumErrorCorrectionBlocks[q.ECC][q.Version]
	blockEccLen := ECCCodewordsPerBlock[q.ECC][q.Version]
	rawCodewords := NumRawDataModules(q.Version) / 8

	numShortBlocks := int(numBlocks) - rawCodewords%int(numBlocks)
	shortBlockLen := rawCodewords / int(numBlocks)

	blocks := make([][]byte, numBlocks)
	rsDiv, err := reedsolomon.Divisor(int(blockEccLen))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errs.ErrInvalidArgument, err)
	}
	for i, k := 0, 0; i < int(numBlocks); i++ {
		index := 1
		if i < numShortBlocks {
			index = 0
		}

		dat := make([]byte, shortBlockLen-int(blockEccLen)+index)
		copy(dat, data[k:k+shortBlockLen-int(blockEccLen)+index])
		k += len(dat)

		block := make([]byte, shortBlockLen+1)
		copy(block, dat)

		ecc := reedsolomon.Remainder(dat, rsDiv)
		copy(block[len(block)-int(blockEccLen):], ecc)
		blocks[i] = block
	}

	res := make([]byte, rawCodewords)
	for i, k := 0, 0; i < len(blocks[0]); i++ {
		for j := 0; j < len(blocks); j++ {
			if i != shortBlockLen-int(blockEccLen) || j >= numShortBlocks {
				res[k] = blocks[j][i]
				k++
			}
		}
	}
	return res, nil
}

// drawCodewords fills the non-function modules with the given codeword bytes
// in the zig-zag order of the standard. The version's template records the
// data bit of each module, so the modules are filled row by row, and each
// row is packed into q.packed for mask selection as it is filled.
func (q *Builder) DrawCodewords(data []byte) error {
	n := NumRawDataModules(q.Version) / 8
	if len(data) != n {
		return fmt.Errorf("%w: codeword length mismatch", errs.ErrInvalidArgument)
	}

	tmpl := getTemplate(q.Version)
	q.packed = make([]moduleLine, 4*q.Size)
	for y, row := range q.Modules {
		bitAt := tmpl.bitAt[y][:len(row)]
		line := tmpl.functionRows[y]
		for w := 0; w*64 < len(row); w++ {
			acc := line[w]
			for x := w * 64; x < min(len(row), w*64+64); x++ {
				i := bitAt[x]
				if i < 0 {
					continue
				}
				dark := data[i>>3]>>(7-i&7)&1 != 0
				row[x] = dark
				acc |= b2u(dark) << (x & 63)
			}
			line[w] = acc
		}
		q.packed[y] = line
	}
	return nil
}

// drawAlignmentPattern draws an alignment pattern centered at (x, y).
func (q *Builder) drawAlignmentPattern(x, y int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			q.setFunctionModule(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

// drawFinderPattern draws a finder pattern centered at (x, y).
func (q *Builder) drawFinderPattern(x, y int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := max(abs(dx), abs(dy))
			xx, yy := x+dx, y+dy
			if 0 <= xx && xx < q.Size && 0 <= yy && yy < q.Size {
				q.setFunctionModule(xx, yy, dist != 2 && dist != 4)
			}
		}
	}
}

// drawVersion encodes version information; only emitted for version >= 7.
func (q *Builder) drawVersion() {
	if q.Version < 7 {
		return
	}

	bits := VersionBits(q.Version)

	for i := 0; i < 18; i++ {
		bit := getBit(bits, i)
		a := q.Size - 11 + i%3
		b := i / 3
		q.setFunctionModule(a, b, bit)
		q.setFunctionModule(b, a, bit)
	}
}

// drawFunctionPatterns draws all static patterns: finders, timing patterns,
// alignment patterns, format/version info placeholders.
func (q *Builder) DrawFunctionPatterns() {
	// Timing patterns
	for i := 0; i < q.Size; i++ {
		q.setFunctionModule(6, i, i%2 == 0)
		q.setFunctionModule(i, 6, i%2 == 0)
	}

	// Finder patterns
	q.drawFinderPattern(3, 3)
	q.drawFinderPattern(q.Size-4, 3)
	q.drawFinderPattern(3, q.Size-4)

	// Alignment patterns
	alignPatPos := q.getAlignmentPatternPositions()
	numAlign := len(alignPatPos)
	for i := 0; i < numAlign; i++ {
		for j := 0; j < numAlign; j++ {
			if !(i == 0 && j == 0 || i == 0 && j == numAlign-1 || i == numAlign-1 && j == 0) {
				q.drawAlignmentPattern(alignPatPos[i], alignPatPos[j])
			}
		}
	}

	q.DrawFormatBits(0)
	q.drawVersion()
}

// getAlignmentPatternPositions returns the alignment pattern center coordinates
// for the QR Code version. For version 1 the result is empty.
func (q *Builder) getAlignmentPatternPositions() []int {
	return AlignmentPositions(q.Version)
}

// AlignmentPositions returns the row and column coordinates of the
// alignment pattern centers of a version (ISO/IEC 18004 Annex E); patterns
// sit at every combination except the three finder corners. Version 1 has
// none.
func AlignmentPositions(version int) []int {
	if version == 1 {
		return []int{}
	}
	numAlign := version/7 + 2
	step := 0
	if version == 32 {
		step = 26
	} else {
		step = (version*4 + numAlign*2 + 1) / (numAlign*2 - 2) * 2
	}

	res := make([]int, numAlign)
	res[0] = 6
	for i, pos := len(res)-1, 4*version+17-7; i >= 1; {
		res[i] = pos
		i--
		pos -= step
	}

	return res
}

// drawFormatBits encodes the ECC level and mask number into the format bits.
func (q *Builder) DrawFormatBits(msk int) {
	data := FormatBits(q.ECC)<<3 | msk
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}

	bits := (data<<10 | rem) ^ 0x5412

	for i := 0; i <= 5; i++ {
		q.setFunctionModule(8, i, getBit(bits, i))
	}
	q.setFunctionModule(8, 7, getBit(bits, 6))
	q.setFunctionModule(8, 8, getBit(bits, 7))
	q.setFunctionModule(7, 8, getBit(bits, 8))

	for i := 9; i < 15; i++ {
		q.setFunctionModule(14-i, 8, getBit(bits, i))
	}

	for i := 0; i < 8; i++ {
		q.setFunctionModule(q.Size-1-i, 8, getBit(bits, i))
	}

	for i := 8; i < 15; i++ {
		q.setFunctionModule(8, q.Size-15+i, getBit(bits, i))
	}
	q.setFunctionModule(8, q.Size-8, true)
}
