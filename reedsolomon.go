package go_qr

import "fmt"

// getNumRawDataModules calculates the number of raw data modules for a specific QR code version.
func getNumRawDataModules(ver int) int {
	// Calculate the size of the QR code grid.
	// For each version, the size increases by 4 modules.
	size := ver*4 + 17

	// Start with the total number of modules in the QR code grid (size^2)
	res := size * size

	// Subtract the three position detection patterns (each is 8x8 modules)
	res -= 8 * 8 * 3

	// Subtract the two horizontal timing patterns and the two vertical timing patterns
	// (each is 15 modules long), along with the single dark module reserved for format information
	res -= 15*2 + 1

	// Subtract the border modules around the timing patterns
	res -= (size - 16) * 2

	// If version is 2 or higher, there are alignment patterns
	if ver >= 2 {
		// Get the number of alignment patterns for this version of QR code
		numAlign := ver/7 + 2

		// Subtract the space taken up by the alignment patterns (each is 5x5 modules)
		res -= (numAlign - 1) * (numAlign - 1) * 25

		// Subtract the two sets of border modules around the alignment patterns
		res -= (numAlign - 2) * 2 * 20

		// For versions 7 and above, subtract the space for version information (6x3 modules on both sides)
		if ver >= 7 {
			res -= 6 * 3 * 2
		}
	}
	return res
}

// reedSolomonComputeDivisor computes a Reed-Solomon divisor for a given degree.
// The degree must be between 1 and 255 inclusive and determines the size of the output byte slice.
// The Reed-Solomon divisor computed by this function is used in error detection and correction codes.
func reedSolomonComputeDivisor(degree int) ([]byte, error) {
	if degree < 1 || degree > 255 {
		return nil, fmt.Errorf("%w: degree %d out of range [1,255]", ErrInvalidArgument, degree)
	}

	res := make([]byte, degree)
	res[degree-1] = 1

	root := 1
	for i := 0; i < degree; i++ {
		for j := 0; j < len(res); j++ {
			// Multiply the jth element of res by root using Reed-Solomon multiplication
			res[j] = byte(reedSolomonMultiply(int(res[j]&0xFF), root))
			if j+1 < len(res) {
				res[j] ^= res[j+1]
			}
		}
		root = reedSolomonMultiply(root, 0x02)
	}
	return res, nil
}

// reedSolomonComputeRemainder computes the remainder of Reed-Solomon encoding.
// Reed-Solomon is an error correction technique used in QR codes (and other data storage).
// This function takes two parameters: data and divisor which are both slices of bytes.
func reedSolomonComputeRemainder(data, divisor []byte) []byte {
	res := make([]byte, len(divisor))
	for _, b := range data {
		factor := (b ^ res[0]) & 0xFF
		copy(res, res[1:])
		res[len(res)-1] = byte(0)
		for i := 0; i < len(res); i++ {
			res[i] ^= byte(reedSolomonMultiply(int(divisor[i]&0xFF), int(factor)))
		}
	}
	return res
}

// reedSolomonMultiply performs multiplication in Galois Field 2^8.
func reedSolomonMultiply(x, y int) int {
	z := 0
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= ((y >> i) & 1) * x
	}
	return z
}
