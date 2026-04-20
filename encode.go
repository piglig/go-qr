package go_qr

import "fmt"

// EncodeText takes a string and an error correction level (ecl),
// encodes the text to segments and returns a QR code or an error.
func EncodeText(text string, ecl Ecc) (*QrCode, error) {
	segs, err := MakeSegments(text)
	if err != nil {
		return nil, err
	}

	return EncodeStandardSegments(segs, ecl)
}

// EncodeBinary takes a byte array and an error correction level (ecl),
// converts the bytes to QR code segments and returns a QR code or an error.
func EncodeBinary(data []byte, ecl Ecc) (*QrCode, error) {
	segs, err := MakeBytes(data)
	if err != nil {
		return nil, err
	}

	return EncodeStandardSegments([]*QrSegment{segs}, ecl)
}

// EncodeStandardSegments takes QR code segments and an error correction level,
// creates a standard QR code using these parameters and returns it or an error.
func EncodeStandardSegments(segs []*QrSegment, ecl Ecc) (*QrCode, error) {
	return EncodeSegments(segs, ecl, MinVersion, MaxVersion, -1, true)
}

// EncodeSegments is a more flexible version of EncodeStandardSegments. It allows
// the specification of minVer, maxVer, mask in addition to the regular parameters.
// Returns a QR code object or an error.
func EncodeSegments(segs []*QrSegment, ecl Ecc, minVer, maxVer, mask int, boostEcl bool) (*QrCode, error) {
	if segs == nil {
		return nil, fmt.Errorf("%w: segments slice is nil", ErrInvalidArgument)
	}

	if !isValidVersion(minVer, maxVer) {
		return nil, fmt.Errorf("%w: minVer=%d maxVer=%d", ErrInvalidVersion, minVer, maxVer)
	}

	// Loop over all versions between minVer and maxVer to find a suitable one
	version, dataUsedBits := 0, 0
	for version = minVer; ; version++ {
		dataCapacityBits := getNumDataCodewords(version, ecl) * 8
		dataUsedBits = getTotalBits(segs, version)
		if dataUsedBits != -1 && dataUsedBits <= dataCapacityBits {
			break
		}

		if version >= maxVer {
			msg := "Segment too long"
			if dataUsedBits != -1 {
				msg = fmt.Sprintf("Data length = %d bits, Max capacity = %d bits", dataUsedBits, dataCapacityBits)
			}
			return nil, &DataTooLongException{Msg: msg}
		}
	}

	// If boostEcl is set, upgrade ECC as far as the data still fits.
	for _, newEcl := range []Ecc{Medium, Quartile, High} {
		numDataCodewords := getNumDataCodewords(version, newEcl)
		if boostEcl && dataUsedBits <= numDataCodewords*8 {
			ecl = newEcl
		}
	}

	bb := BitBuffer{}
	for _, seg := range segs {
		if seg == nil {
			continue
		}

		err := bb.appendBits(seg.mode.modeBits, 4)
		if err != nil {
			return nil, err
		}
		err = bb.appendBits(seg.numChars, seg.mode.numCharCountBits(version))
		if err != nil {
			return nil, err
		}
		err = bb.appendData(seg.data)
		if err != nil {
			return nil, err
		}
	}

	dataCapacityBits := getNumDataCodewords(version, ecl) * 8
	err := bb.appendBits(0, min(4, dataCapacityBits-bb.len()))
	if err != nil {
		return nil, err
	}

	err = bb.appendBits(0, (8-bb.len()%8)%8)
	if err != nil {
		return nil, err
	}

	for padByte := 0xEC; bb.len() < dataCapacityBits; padByte ^= 0xEC ^ 0x11 {
		err = bb.appendBits(padByte, 8)
		if err != nil {
			return nil, err
		}
	}

	dataCodewords := make([]byte, bb.len()/8)
	for i := 0; i < bb.len(); i++ {
		bit := 0
		if bb.getBit(i) {
			bit = 1
		}
		dataCodewords[i>>3] |= byte(bit << (7 - (i & 7)))
	}
	return newQrCode(version, ecl, dataCodewords, mask)
}

// isValidVersion reports whether minVer and maxVer lie within [MinVersion, MaxVersion] and minVer <= maxVer.
func isValidVersion(minVer, maxVer int) bool {
	return MinVersion <= minVer && minVer <= maxVer && maxVer <= MaxVersion
}

// getNumDataCodewords returns the number of data codewords for a given version and ECC level.
func getNumDataCodewords(ver int, ecl Ecc) int {
	eccCodewordsPerBlock := getEccCodeWordsPerBlock()
	numRawDataModules := getNumRawDataModules(ver)
	numErrorCorrectionBlocks := getNumErrorCorrectionBlocks()
	return numRawDataModules/8 -
		int(eccCodewordsPerBlock[ecl][ver])*int(numErrorCorrectionBlocks[ecl][ver])
}
