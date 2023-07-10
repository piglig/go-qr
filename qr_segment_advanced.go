package go_qr

import (
	"errors"
	"unicode/utf16"
)

type QrSegmentAdvanced struct {
}

func MakeSegmentsOptimally(text string, ecl Ecc, minVersion, maxVersion int) ([]*QrSegment, error) {
	if !(MinVersion <= minVersion && minVersion <= maxVersion && maxVersion <= MaxVersion) {
		return nil, errors.New("invalid value")
	}

	return nil, nil
}

// Returns a new slice of Unicode code points (effectively
// UTF-32 / UCS-4) representing the given UTF-16 string.
func toCodePoints(s string) ([]int, error) {
	runes := []rune(s)
	codePoints := make([]int, len(runes))
	for i, r := range runes {
		if utf16.IsSurrogate(r) {
			return nil, errors.New("invalid UTF-16 string")
		}
		codePoints[i] = int(r)
	}

	return codePoints, nil
}

func countUtf8Bytes(cp int) (int, error) {
	if cp < 0 {
		return 0, errors.New("invalid code point")
	} else if cp < 0x80 {
		return 1, nil
	} else if cp < 0x800 {
		return 2, nil
	} else if cp < 0x10000 {
		return 3, nil
	} else if cp < 0x110000 {
		return 4, nil
	} else {
		return 0, errors.New("invalid code point")
	}
}
