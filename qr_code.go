package go_qr

import "fmt"

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

// eccCodeWordsPerBlock is a lookup table for the number of error correction
// code words per block, indexed by error correction level and version.
var eccCodeWordsPerBlock = [][]int8{
	// Version: (note that index 0 is for padding, and is set to an illegal value)
	//0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
	{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},  // Low
	{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28}, // Medium
	{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // Quartile
	{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // High
}

// numErrorCorrectionBlocks is a lookup table for the number of error
// correction blocks, indexed by error correction level and version.
var numErrorCorrectionBlocks = [][]int8{
	// Version: (note that index 0 is for padding, and is set to an illegal value)
	//0, 1, 2, 3, 4, 5, 6, 7, 8, 9,10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
	{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},              // Low
	{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},     // Medium
	{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},  // Quartile
	{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81}, // High
}

// QrCode is an immutable, fully-rendered QR Code symbol. Build one with
// EncodeText / EncodeSegments; query it with Size and Module; render it with the
// PNG/SVG methods. It carries no build-time scratch (see builder).
type QrCode struct {
	version              int      // Version of the QR Code.
	size                 int      // Side length in modules.
	errorCorrectionLevel Ecc      // Error correction level (ECC).
	mask                 int      // Mask pattern applied.
	modules              [][]bool // Dark/light state of every module (read-only).
}

// newQrCode encodes the data codewords into a finished QrCode at the given
// version and ECC level. msk selects the mask pattern; -1 chooses the
// lowest-penalty mask. It drives a builder and freezes the result.
func newQrCode(ver int, ecl Ecc, dataCodewords []byte, msk int) (*QrCode, error) {
	if msk < -1 || msk > 7 {
		return nil, fmt.Errorf("%w: mask value out of range", ErrInvalidArgument)
	}

	b := newBuilder(ver, ecl)
	b.drawFunctionPatterns()

	allCodewords, err := b.addEccAndInterLeave(dataCodewords)
	if err != nil {
		return nil, err
	}
	if err := b.drawCodewords(allCodewords); err != nil {
		return nil, err
	}

	if msk == -1 {
		msk = b.chooseBestMask()
	}
	if err := b.applyMask(msk); err != nil {
		return nil, err
	}
	b.drawFormatBits(msk)

	return b.toQrCode(msk), nil
}

// Size returns the side length of the QR code in modules.
func (q *QrCode) Size() int {
	return q.size
}

// Module reports whether the module at (x, y) is dark.
func (q *QrCode) Module(x, y int) bool {
	return 0 <= x && x < q.size && 0 <= y && y < q.size && q.modules[y][x]
}

// getBit returns the i-th bit (LSB-first) of x.
func getBit(x, i int) bool {
	return ((x >> uint(i)) & 1) != 0
}
