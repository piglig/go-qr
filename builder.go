package go_qr

import (
	"fmt"
	"math"

	"github.com/piglig/go-qr/internal/reedsolomon"
)

// builder is the mutable scaffold used to lay out a QR matrix. It owns the
// module grid plus the function-module map (isFunction) that drives codeword
// placement and masking. The encoder drives a builder to completion and then
// freezes it into an immutable QrCode; the decoder reuses a builder purely to
// rebuild the function map and undo masking. Keeping this state off QrCode means
// the returned value is immutable and carries no build-time scratch.
type builder struct {
	version              int
	size                 int
	errorCorrectionLevel Ecc
	modules              [][]bool // dark/light state of every module
	isFunction           [][]bool // true where a module belongs to a function pattern
}

// newBuilder allocates a blank builder for the given version and ECC level.
func newBuilder(version int, ecl Ecc) *builder {
	size := version*4 + 17
	b := &builder{version: version, size: size, errorCorrectionLevel: ecl}
	// One backing allocation per grid; rows alias into it.
	b.modules = make([][]bool, size)
	b.isFunction = make([][]bool, size)
	modBacking := make([]bool, size*size)
	fnBacking := make([]bool, size*size)
	for i := 0; i < size; i++ {
		b.modules[i] = modBacking[i*size : (i+1)*size]
		b.isFunction[i] = fnBacking[i*size : (i+1)*size]
	}
	return b
}

// toQrCode freezes the builder into an immutable QrCode with the chosen mask.
// The module grid is handed over directly; the builder must not be used after.
func (q *builder) toQrCode(mask int) *QrCode {
	return &QrCode{
		version:              q.version,
		size:                 q.size,
		errorCorrectionLevel: q.errorCorrectionLevel,
		mask:                 mask,
		modules:              q.modules,
	}
}

// chooseBestMask scores all eight mask patterns and returns the one with the
// lowest penalty. It does not mutate the module grid (masking is scored
// out-of-place); the caller applies the winning mask afterwards.
func (q *builder) chooseBestMask() int {
	minPenalty := math.MaxInt32
	best := 0
	// Reusable masked-grid scratch; rows alias one backing allocation.
	scratch := make([][]bool, q.size)
	scratchBacking := make([]bool, q.size*q.size)
	for r := 0; r < q.size; r++ {
		scratch[r] = scratchBacking[r*q.size : (r+1)*q.size]
	}
	// Version-invariant mask patterns, built once per version and reused across
	// all encodes of that version (e.g. a batch of similar payloads).
	tmpl := getTemplate(q.version)
	for i := 0; i < 8; i++ {
		q.drawFormatBits(i)
		q.writeMaskedGrid(scratch, tmpl.maskPatterns[i])
		penalty := q.getPenaltyScore(scratch, minPenalty)
		if penalty < minPenalty {
			best = i
			minPenalty = penalty
		}
	}
	return best
}

// setFunctionModule paints a module and marks it as part of a function pattern.
func (q *builder) setFunctionModule(x, y int, isDark bool) {
	q.modules[y][x] = isDark
	q.isFunction[y][x] = true
}

// addEccAndInterLeave appends Reed-Solomon ECC bytes to the raw data and
// interleaves the result according to the block layout for the QR version and
// ECC level.
func (q *builder) addEccAndInterLeave(data []byte) ([]byte, error) {
	numDataCodewords := getNumDataCodewords(q.version, q.errorCorrectionLevel)
	if len(data) != numDataCodewords {
		return nil, fmt.Errorf("%w: data length %d != expected %d", ErrInvalidArgument, len(data), numDataCodewords)
	}

	numBlocks := numErrorCorrectionBlocks[q.errorCorrectionLevel][q.version]
	blockEccLen := eccCodeWordsPerBlock[q.errorCorrectionLevel][q.version]
	rawCodewords := getNumRawDataModules(q.version) / 8

	numShortBlocks := int(numBlocks) - rawCodewords%int(numBlocks)
	shortBlockLen := rawCodewords / int(numBlocks)

	blocks := make([][]byte, numBlocks)
	rsDiv, err := reedsolomon.Divisor(int(blockEccLen))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
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
// following the QR Code zig-zag traversal.
func (q *builder) drawCodewords(data []byte) error {
	numRawDataModules := getNumRawDataModules(q.version) / 8
	if len(data) != numRawDataModules {
		return fmt.Errorf("%w: codeword length mismatch", ErrInvalidArgument)
	}

	i := 0
	for right := q.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := ((right + 1) & 2) == 0
				y := vert
				if upward {
					y = q.size - 1 - vert
				}
				if !q.isFunction[y][x] && i < len(data)*8 {
					q.modules[y][x] = getBit(int(data[i>>3]), 7-(i&7))
					i++
				}
			}
		}
	}
	return nil
}

// drawAlignmentPattern draws an alignment pattern centered at (x, y).
func (q *builder) drawAlignmentPattern(x, y int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			q.setFunctionModule(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

// drawFinderPattern draws a finder pattern centered at (x, y).
func (q *builder) drawFinderPattern(x, y int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := max(abs(dx), abs(dy))
			xx, yy := x+dx, y+dy
			if 0 <= xx && xx < q.size && 0 <= yy && yy < q.size {
				q.setFunctionModule(xx, yy, dist != 2 && dist != 4)
			}
		}
	}
}

// drawVersion encodes version information; only emitted for version >= 7.
func (q *builder) drawVersion() {
	if q.version < 7 {
		return
	}

	rem := q.version
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	bits := q.version<<12 | rem

	for i := 0; i < 18; i++ {
		bit := getBit(bits, i)
		a := q.size - 11 + i%3
		b := i / 3
		q.setFunctionModule(a, b, bit)
		q.setFunctionModule(b, a, bit)
	}
}

// drawFunctionPatterns draws all static patterns: finders, timing patterns,
// alignment patterns, format/version info placeholders.
func (q *builder) drawFunctionPatterns() {
	// Timing patterns
	for i := 0; i < q.size; i++ {
		q.setFunctionModule(6, i, i%2 == 0)
		q.setFunctionModule(i, 6, i%2 == 0)
	}

	// Finder patterns
	q.drawFinderPattern(3, 3)
	q.drawFinderPattern(q.size-4, 3)
	q.drawFinderPattern(3, q.size-4)

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

	q.drawFormatBits(0)
	q.drawVersion()
}

// getAlignmentPatternPositions returns the alignment pattern center coordinates
// for the QR Code version. For version 1 the result is empty.
func (q *builder) getAlignmentPatternPositions() []int {
	if q.version == 1 {
		return []int{}
	}
	numAlign := q.version/7 + 2
	step := 0
	if q.version == 32 {
		step = 26
	} else {
		step = (q.version*4 + numAlign*2 + 1) / (numAlign*2 - 2) * 2
	}

	res := make([]int, numAlign)
	res[0] = 6
	for i, pos := len(res)-1, q.size-7; i >= 1; {
		res[i] = pos
		i--
		pos -= step
	}

	return res
}

// drawFormatBits encodes the ECC level and mask number into the format bits.
func (q *builder) drawFormatBits(msk int) {
	data := q.errorCorrectionLevel.FormatBits()<<3 | msk
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
		q.setFunctionModule(q.size-1-i, 8, getBit(bits, i))
	}

	for i := 8; i < 15; i++ {
		q.setFunctionModule(8, q.size-15+i, getBit(bits, i))
	}
	q.setFunctionModule(8, q.size-8, true)
}
