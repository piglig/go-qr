package go_qr

// getNumRawDataModules returns the number of raw data modules (bits available
// for data + ECC codewords, divided by 8 elsewhere) for a QR code version.
func getNumRawDataModules(ver int) int {
	// Total modules in the size×size grid.
	size := ver*4 + 17
	res := size * size

	// Subtract the three 8×8 finder-pattern regions (incl. separators).
	res -= 8 * 8 * 3

	// Subtract the two timing patterns (15 modules each) and the dark module.
	res -= 15*2 + 1

	// Subtract the timing-pattern border modules.
	res -= (size - 16) * 2

	// Alignment patterns and version info exist from version 2 / 7 up.
	if ver >= 2 {
		numAlign := ver/7 + 2
		res -= (numAlign - 1) * (numAlign - 1) * 25
		res -= (numAlign - 2) * 2 * 20
		if ver >= 7 {
			res -= 6 * 3 * 2
		}
	}
	return res
}
