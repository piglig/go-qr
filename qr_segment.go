package go_qr

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Mode is the representation of the mode of a QR code character. It is
// comparable, so modes can be checked with ==.
type Mode struct {
	modeBits         int    // 4-bit mode indicator used in QR Code's data encoding
	numBitsCharCount [3]int // char-count-indicator bits for the three version ranges
}

// newMode creates a new Mode with a given mode indicator and the character count
// bits for the three version ranges (1-9, 10-26, 27-40).
func newMode(mode, cc1, cc2, cc3 int) Mode {
	return Mode{
		modeBits:         mode,
		numBitsCharCount: [3]int{cc1, cc2, cc3},
	}
}

// numCharCountBits returns the number of character count bits
// for a specific QR code version.
func (m Mode) numCharCountBits(ver int) int {
	return m.numBitsCharCount[(ver+7)/17]
}

// Predefined Mode values as defined by the QR Code standard.
var (
	// Numeric mode is typically used for decimal digits (0 through 9).
	Numeric = newMode(0x1, 10, 12, 14)

	// Alphanumeric mode includes digits 0-9, uppercase letters A-Z and nine special characters.
	Alphanumeric = newMode(0x2, 9, 11, 13)

	// Byte mode can encode binary/byte data(default: ISO-8859-1)
	Byte = newMode(0x4, 8, 16, 16) // Byte mode: binary/byte data (default: ISO-8859-1)

	// Kanji mode is used for encoding Japanese Kanji characters.
	Kanji = newMode(0x8, 8, 10, 12)

	// Eci mode is designed for providing a method of extending features and functions
	// in bar code symbols beyond those envisioned by the original standard.
	Eci = newMode(0x7, 0, 0, 0)
)

// bits returns the 4-bit mode indicator.
func (m Mode) bits() int {
	return m.modeBits
}

// isNumeric checks if the mode is Numeric.
func (m Mode) isNumeric() bool { return m == Numeric }

// isAlphanumeric checks if the mode is Alphanumeric.
func (m Mode) isAlphanumeric() bool { return m == Alphanumeric }

// isByte checks if the mode is Byte.
func (m Mode) isByte() bool { return m == Byte }

// isKanji checks if the mode is Kanji.
func (m Mode) isKanji() bool { return m == Kanji }

// isEci checks if the mode is ECI.
func (m Mode) isEci() bool { return m == Eci }

// alphanumericCharset lists every character encodable in alphanumeric mode; the
// index of a character is also its alphanumeric value.
const alphanumericCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

// QrSegment is the representation of a segment of a QR code.
type QrSegment struct {
	mode     Mode       // The mode of the QR segment (e.g., Numeric, Alphanumeric, etc.)
	numChars int        // Number of characters in the segment
	data     *BitBuffer // The actual binary data represented by this QR segment
}

// newQrSegment function creates a new QR segment with the given mode, number of characters, and data.
func newQrSegment(mode Mode, numCh int, data *BitBuffer) (*QrSegment, error) {
	if numCh < 0 {
		return nil, fmt.Errorf("%w: numChars %d is negative", ErrInvalidArgument, numCh)
	}
	return &QrSegment{
		mode:     mode,
		numChars: numCh,
		data:     data.clone(), // Use a cloned copy of the data to prevent modifications
	}, nil
}

// cloneData returns a copy of the segment's BitBuffer data.
func (q *QrSegment) cloneData() *BitBuffer {
	return q.data.clone()
}

// MakeBytes converts a byte slice into a QR segment in Byte mode.
// It returns an error if the input data is nil.
func MakeBytes(data []byte) (*QrSegment, error) {
	if data == nil {
		return nil, fmt.Errorf("%w: data is nil", ErrInvalidArgument)
	}

	bb := &BitBuffer{}
	for _, b := range data {
		err := bb.appendBits(int(b&0xFF), 8) // Append 8 bits at once to the bit buffer
		if err != nil {
			return nil, err
		}
	}
	return newQrSegment(Byte, len(data), bb)
}

// MakeNumeric converts a string of digits into a QR code segment in Numeric mode
// It returns an error if the string contains non-numeric characters.
func MakeNumeric(digits string) (*QrSegment, error) {
	if !isNumeric(digits) {
		return nil, fmt.Errorf("%w: numeric mode", ErrUnencodableChar)
	}

	bb := &BitBuffer{}
	for i := 0; i < len(digits); {
		n := min(len(digits)-i, 3)              // find the length of the current chunk (up to 3 digits)
		num, _ := strconv.Atoi(digits[i : i+n]) // convert the current chunk to an integer
		err := bb.appendBits(num, n*3+1)
		if err != nil {
			return nil, err
		}
		i += n // advance to the next chunk
	}

	return newQrSegment(Numeric, len(digits), bb)
}

// isNumeric reports whether s is non-empty and contains only ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isAlphanumeric reports whether every byte of s is encodable in alphanumeric
// mode (digits, uppercase letters, and the nine symbols in alphanumericCharset).
func isAlphanumeric(s string) bool {
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(alphanumericCharset, s[i]) < 0 {
			return false
		}
	}
	return true
}

// MakeAlphanumeric converts a string into a QR code segment in Alphanumeric mode
// It returns an error if the string contains non-alphanumeric characters.
func MakeAlphanumeric(text string) (*QrSegment, error) {
	if !isAlphanumeric(text) {
		return nil, fmt.Errorf("%w: alphanumeric mode", ErrUnencodableChar)
	}

	bb := &BitBuffer{}
	i := 0
	for ; i <= len(text)-2; i += 2 {
		// Process each pair of characters in text.
		temp := strings.IndexByte(alphanumericCharset, text[i]) * 45
		temp += strings.IndexByte(alphanumericCharset, text[i+1])
		err := bb.appendBits(temp, 11)
		if err != nil {
			return nil, err
		}
	}

	if i < len(text) {
		err := bb.appendBits(strings.IndexByte(alphanumericCharset, text[i]), 6)
		if err != nil {
			return nil, err
		}
	}

	return newQrSegment(Alphanumeric, len(text), bb)
}

// MakeSegments converts data into QR segments based on the mode of text (Numeric, Alphanumeric or Byte, etc).
func MakeSegments(text string) ([]*QrSegment, error) {
	res := make([]*QrSegment, 0)
	if text == "" {
	} else if isNumeric(text) {
		seg, err := MakeNumeric(text)
		if err != nil {
			return nil, err
		}

		res = append(res, seg)
	} else if isAlphanumeric(text) {
		seg, err := MakeAlphanumeric(text)
		if err != nil {
			return nil, err
		}
		res = append(res, seg)
	} else {
		seg, err := MakeBytes([]byte(text))
		if err != nil {
			return nil, err
		}
		res = append(res, seg)
	}
	return res, nil
}

// MakeEci converts an integer into a QR code segment in Eci mode
// It returns an error if the integer value is out of range.
func MakeEci(val int) (*QrSegment, error) {
	bb := &BitBuffer{}
	if val < 0 {
		return nil, fmt.Errorf("%w: ECI assignment value out of range", ErrInvalidArgument)
	} else if val < (1 << 7) {
		err := bb.appendBits(val, 8)
		if err != nil {
			return nil, err
		}
	} else if val < (1 << 14) {
		err := bb.appendBits(0b10, 2)
		if err != nil {
			return nil, err
		}

		err = bb.appendBits(val, 14)
		if err != nil {
			return nil, err
		}
	} else if val < 1e6 {
		err := bb.appendBits(0b110, 3)
		if err != nil {
			return nil, err
		}

		err = bb.appendBits(val, 21)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("%w: ECI assignment value out of range", ErrInvalidArgument)
	}
	return newQrSegment(Eci, 0, bb)
}

// getTotalBits calculates and returns the total number of bits required to encode the segments at the specified QR version.
// It returns -1 if the number of characters exceeds the maximum capacity.
func getTotalBits(segs []*QrSegment, ver int) int {
	var res int64
	for _, seg := range segs {
		if seg == nil {
			continue
		}

		ccbits := seg.mode.numCharCountBits(ver)
		if seg.numChars >= (1 << ccbits) {
			return -1
		}
		res += int64(4 + ccbits + seg.data.len())
		if res > math.MaxInt32 {
			return -1
		}
	}
	return int(res)
}
