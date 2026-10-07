package qr

import "fmt"

// ECC is an error correction level. Higher levels recover from more damage at
// the cost of capacity.
type ECC int8

const (
	ECCLow      ECC = iota // recovers about 7% of codewords
	ECCMedium              // recovers about 15% of codewords
	ECCQuartile            // recovers about 25% of codewords
	ECCHigh                // recovers about 30% of codewords
)

// String returns "L", "M", "Q" or "H".
func (e ECC) String() string {
	switch e {
	case ECCLow:
		return "L"
	case ECCMedium:
		return "M"
	case ECCQuartile:
		return "Q"
	case ECCHigh:
		return "H"
	}
	return fmt.Sprintf("ECC(%d)", int8(e))
}

func (e ECC) valid() bool { return ECCLow <= e && e <= ECCHigh }

// formatBits returns the 2-bit ECC indicator used in the format information.
func (e ECC) formatBits() int { return eccFormatBits[e] }

// eccFormatBits maps an ECC level to its format indicator (ISO/IEC 18004
// Table 12). The mapping is an involution, so it also decodes.
var eccFormatBits = [...]int{1, 0, 3, 2}

// MinVersion and MaxVersion bound the QR Code Model 2 versions (symbol sizes
// 21×21 to 177×177 modules).
const (
	MinVersion = 1
	MaxVersion = 40
)

var eccCodeWordsPerBlock = [4][41]int8{
	// Version: (note that index 0 is for padding, and is set to an illegal value)
	//0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
	{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},  // ECCLow
	{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28}, // ECCMedium
	{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // ECCQuartile
	{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // ECCHigh
}
var numErrorCorrectionBlocks = [4][41]int8{
	// Version: (note that index 0 is for padding, and is set to an illegal value)
	//0, 1, 2, 3, 4, 5, 6, 7, 8, 9,10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
	{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},              // ECCLow
	{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},     // ECCMedium
	{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},  // ECCQuartile
	{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81}, // ECCHigh
}

// Code is an immutable QR Code symbol. Create one with Encode, EncodeBytes or
// EncodeSegments; query it with Size and Module; render it with the Write*
// methods.
type Code struct {
	version int
	size    int
	ecc     ECC
	mask    int
	modules [][]bool // modules[y][x] reports whether the module is dark
}

// newCode encodes the data codewords into a finished Code at the given version
// and ECC level. mask selects the mask pattern; -1 chooses the lowest-penalty
// mask.
func newCode(ver int, ecc ECC, dataCodewords []byte, mask int) (*Code, error) {
	b := newBuilder(ver, ecc)
	b.drawFunctionPatterns()

	allCodewords, err := b.addEccAndInterLeave(dataCodewords)
	if err != nil {
		return nil, err
	}
	if err := b.drawCodewords(allCodewords); err != nil {
		return nil, err
	}

	if mask == -1 {
		mask = b.chooseBestMask()
	}
	if err := b.applyMask(mask); err != nil {
		return nil, err
	}
	b.drawFormatBits(mask)

	return b.toCode(mask), nil
}

// Size returns the side length of the symbol in modules (4*Version()+17). It
// does not include the quiet zone.
func (c *Code) Size() int { return c.size }

// Version returns the symbol version, from MinVersion to MaxVersion.
func (c *Code) Version() int { return c.version }

// ECC returns the error correction level of the symbol. It can be higher than
// the requested level, see WithoutECCBoost.
func (c *Code) ECC() ECC { return c.ecc }

// Mask returns the mask pattern applied to the symbol, from 0 to 7.
func (c *Code) Mask() int { return c.mask }

// Module reports whether the module at (x, y) is dark. Coordinates outside the
// symbol report false (light), which matches the quiet zone.
func (c *Code) Module(x, y int) bool {
	return 0 <= x && x < c.size && 0 <= y && y < c.size && c.modules[y][x]
}

// getBit returns the i-th bit (LSB-first) of x.
func getBit(x, i int) bool {
	return (x>>uint(i))&1 != 0
}
