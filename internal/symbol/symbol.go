// Package symbol holds the structure of QR Code Model 2 symbols (ISO/IEC
// 18004) that the encoder and the decoder share: the capacity and block
// tables, the function patterns, codeword placement and interleaving, the
// masks and their penalty, format and version information, and reading a
// sampled module grid back into corrected data codewords.
package symbol

// Level is an error correction level, numbered as package qr's ECC.
type Level int8

// The error correction levels.
const (
	Low Level = iota
	Medium
	Quartile
	High
)

// FormatBits returns the 2-bit indicator of l in the format information.
func FormatBits(l Level) int { return eccFormatBits[l] }

// eccFormatBits maps an ECC level to its format indicator (ISO/IEC 18004
// Table 12). The mapping is an involution, so it also decodes.
var eccFormatBits = [...]int{1, 0, 3, 2}

// MinVersion and MaxVersion bound the QR Code Model 2 versions (symbol sizes
// 21×21 to 177×177 modules).
const (
	MinVersion = 1
	MaxVersion = 40
)

var ECCCodewordsPerBlock = [4][41]int8{
	// Version: (note that index 0 is for padding, and is set to an illegal value)
	//0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
	{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},  // Low
	{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28}, // Medium
	{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // Quartile
	{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // High
}

var NumErrorCorrectionBlocks = [4][41]int8{
	// Version: (note that index 0 is for padding, and is set to an illegal value)
	//0, 1, 2, 3, 4, 5, 6, 7, 8, 9,10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
	{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},              // Low
	{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},     // Medium
	{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},  // Quartile
	{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81}, // High
}

// getBit returns the i-th bit (LSB-first) of x.
func getBit(x, i int) bool {
	return (x>>uint(i))&1 != 0
}

// CorrectableErrors returns how many codeword errors each block of a symbol
// can correct: half its error correction codewords, less the
// misdecode protection codewords p of ISO/IEC 18004 Table 9, which the
// smallest symbols reserve for detecting errors rather than correcting
// them. Readers that follow the standard correct no more.
func CorrectableErrors(ver int, ecc Level) int {
	return (int(ECCCodewordsPerBlock[ecc][ver]) - MisdecodeProtection(ver, ecc)) / 2
}

// MisdecodeProtection returns the misdecode protection codewords p of
// ISO/IEC 18004 Table 9: check bytes of each block that are not for
// correcting errors but for detecting a correction gone wrong.
func MisdecodeProtection(ver int, ecc Level) int {
	switch {
	case ver == 1 && ecc == Low:
		return 3
	case ver == 1 && ecc == Medium, ver == 2 && ecc == Low:
		return 2
	case ver == 1, ver == 3 && ecc == Low:
		return 1
	}
	return 0
}

// VersionBits returns the 18-bit version information codeword: the version
// followed by its BCH(18,6) remainder (ISO/IEC 18004 Annex D).
func VersionBits(ver int) int {
	rem := ver
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	return ver<<12 | rem
}

// CorrectVersion returns the version whose codeword is nearest to bits, if it
// is within the 3-bit correction capacity of the code.
func CorrectVersion(bits int) (int, bool) {
	best, bestDist := 0, 4
	for v := 7; v <= MaxVersion; v++ {
		if d := bitCount(VersionBits(v) ^ bits); d < bestDist {
			best, bestDist = v, d
		}
	}
	return best, bestDist <= 3
}

// NumDataCodewords returns the number of data (non-ECC) codewords in a symbol
// of the given version and ECC level.
func NumDataCodewords(ver int, ecc Level) int {
	return NumRawDataModules(ver)/8 -
		int(ECCCodewordsPerBlock[ecc][ver])*int(NumErrorCorrectionBlocks[ecc][ver])
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
