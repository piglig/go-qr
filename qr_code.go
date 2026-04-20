package go_qr

import (
	"fmt"
	"math"
)

// Ecc is the representation of an error correction level in a QR Code symbol.
type Ecc int

const (
	Low      Ecc = iota // 7% of codewords can be restored
	Medium              // 15% of codewords can be restored
	Quartile            // 25% of codewords can be restored
	High                // 30% of codewords can be restored
)

// eccFormats maps the ECC to its respective format bits.
var eccFormats = [...]int{1, 0, 3, 2}

// FormatBits returns the format bits associated with the error correction level.
func (e Ecc) FormatBits() int {
	return eccFormats[e]
}

// MinVersion / MaxVersion define the supported QR Code Model 2 version range.
const (
	MinVersion = 1
	MaxVersion = 40
)

// getEccCodeWordsPerBlock is a lookup table for the number of error correction
// code words per block, indexed by error correction level and version.
func getEccCodeWordsPerBlock() [][]int8 {
	return [][]int8{
		// Version: (note that index 0 is for padding, and is set to an illegal value)
		//0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
		{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},  // Low
		{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28}, // Medium
		{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // Quartile
		{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // High
	}
}

// getNumErrorCorrectionBlocks is a lookup table for the number of error
// correction blocks, indexed by error correction level and version.
func getNumErrorCorrectionBlocks() [][]int8 {
	return [][]int8{
		// Version: (note that index 0 is for padding, and is set to an illegal value)
		//0, 1, 2, 3, 4, 5, 6, 7, 8, 9,10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
		{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},              // Low
		{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},     // Medium
		{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},  // Quartile
		{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81}, // High
	}
}

// QrCode is the representation of a QR code.
type QrCode struct {
	version              int // Version of the QR Code.
	size                 int // Size of the QR Code.
	errorCorrectionLevel Ecc // Error correction level (ECC) of the QR Code.
	mask                 int // Mask pattern of the QR Code.

	modules    [][]bool // 2D boolean matrix representing dark modules in the QR Code.
	isFunction [][]bool // 2D boolean matrix distinguishing function from data modules.
}

// newQrCode creates a new QR code with the provided version, ECC level, data
// codewords and mask value (-1 chooses the best mask by minimizing penalty).
func newQrCode(ver int, ecl Ecc, dataCodewords []byte, msk int) (*QrCode, error) {
	if msk < -1 || msk > 7 {
		return nil, fmt.Errorf("%w: mask value out of range", ErrInvalidArgument)
	}

	qrCode := &QrCode{
		version:              ver,
		size:                 ver*4 + 17,
		errorCorrectionLevel: ecl,
	}

	modules := make([][]bool, qrCode.size)
	isFunction := make([][]bool, qrCode.size)
	for i := 0; i < qrCode.size; i++ {
		modules[i] = make([]bool, qrCode.size)
		isFunction[i] = make([]bool, qrCode.size)
	}
	qrCode.modules = modules
	qrCode.isFunction = isFunction

	qrCode.drawFunctionPatterns()

	allCodewords, err := qrCode.addEccAndInterLeave(dataCodewords)
	if err != nil {
		return nil, err
	}

	err = qrCode.drawCodewords(allCodewords)
	if err != nil {
		return nil, err
	}

	// If mask is -1, choose the best mask based on minimum penalty score.
	if msk == -1 {
		minPenalty := math.MaxInt32
		for i := 0; i < 8; i++ {
			err = qrCode.applyMask(i)
			if err != nil {
				return nil, err
			}
			qrCode.drawFormatBits(i)
			penalty := qrCode.getPenaltyScore()
			if penalty < minPenalty {
				msk = i
				minPenalty = penalty
			}
			err = qrCode.applyMask(i)
			if err != nil {
				return nil, err
			}
		}
	}

	qrCode.mask = msk
	err = qrCode.applyMask(msk)
	if err != nil {
		return nil, err
	}

	qrCode.drawFormatBits(msk)
	qrCode.isFunction = nil

	return qrCode, nil
}

// GetSize returns the side length of the QR code in modules.
func (q *QrCode) GetSize() int {
	return q.size
}

// GetModule reports whether the module at (x, y) is dark.
func (q *QrCode) GetModule(x, y int) bool {
	return 0 <= x && x < q.size && 0 <= y && y < q.size && q.modules[y][x]
}

// setFunctionModule paints a module and marks it as part of a function pattern.
func (q *QrCode) setFunctionModule(x, y int, isDark bool) {
	q.modules[y][x] = isDark
	q.isFunction[y][x] = true
}

// addEccAndInterLeave appends Reed-Solomon ECC bytes to the raw data and
// interleaves the result according to the block layout for the QR version and
// ECC level.
func (q *QrCode) addEccAndInterLeave(data []byte) ([]byte, error) {
	numDataCodewords := getNumDataCodewords(q.version, q.errorCorrectionLevel)
	if len(data) != numDataCodewords {
		return nil, fmt.Errorf("%w: data length %d != expected %d", ErrInvalidArgument, len(data), numDataCodewords)
	}

	numBlocks := getNumErrorCorrectionBlocks()[q.errorCorrectionLevel][q.version]
	blockEccLen := getEccCodeWordsPerBlock()[q.errorCorrectionLevel][q.version]
	rawCodewords := getNumRawDataModules(q.version) / 8

	numShortBlocks := int(numBlocks) - rawCodewords%int(numBlocks)
	shortBlockLen := rawCodewords / int(numBlocks)

	blocks := make([][]byte, numBlocks)
	rsDiv, err := reedSolomonComputeDivisor(int(blockEccLen))
	if err != nil {
		return nil, err
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

		ecc := reedSolomonComputeRemainder(dat, rsDiv)
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
func (q *QrCode) drawCodewords(data []byte) error {
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
func (q *QrCode) drawAlignmentPattern(x, y int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			q.setFunctionModule(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

// drawFinderPattern draws a finder pattern centered at (x, y).
func (q *QrCode) drawFinderPattern(x, y int) {
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
func (q *QrCode) drawVersion() {
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
func (q *QrCode) drawFunctionPatterns() {
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
func (q *QrCode) getAlignmentPatternPositions() []int {
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
func (q *QrCode) drawFormatBits(msk int) {
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

// getBit returns the i-th bit (LSB-first) of x.
func getBit(x, i int) bool {
	return ((x >> uint(i)) & 1) != 0
}
