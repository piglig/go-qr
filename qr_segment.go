package go_qr

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Mode is the representation of the mode of a QR code character.
type Mode struct {
	modeBits         int   // 4-bit mode indicator used in QR Code's data encoding
	numBitsCharCount []int // number of bits used for character count indicator for different versions
}

// newMode creates a new Mode with a given mode indicator and character count bits
func newMode(mode int, ccbits ...int) Mode {
	return Mode{
		modeBits:         mode,
		numBitsCharCount: ccbits,
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

// getModeBits function to return the bits representing a particular mode
func (m Mode) getModeBits() int {
	return m.modeBits
}

// isNumeric checks if the mode is Numeric.
func (m Mode) isNumeric() bool {
	return m.modeBits == Numeric.getModeBits()
}

// isAlphanumeric checks if the mode is Alphanumeric.
func (m Mode) isAlphanumeric() bool {
	return m.modeBits == Alphanumeric.getModeBits()
}

// isByte checks if the mode is Byte.
func (m Mode) isByte() bool {
	return m.modeBits == Byte.getModeBits()
}

// isKanji checks if the mode is Kanji.
func (m Mode) isKanji() bool {
	return m.modeBits == Kanji.getModeBits()
}

// isEci checks if the mode is ECI.
func (m Mode) isEci() bool {
	return m.modeBits == Eci.getModeBits()
}

var (
	// numericRegex is a regular expression that matches strings consisting only of numbers (0-9).
	numericRegex = regexp.MustCompile(`^\d+$`)

	// alphanumericRegex is a regular expression that matches strings
	// consisting only of numeric characters, uppercase alphabets, and certain special characters ($%*+-./:).
	alphanumericRegex = regexp.MustCompile(`^[0-9A-Z $%*+\-.\/:]*$`)

	// alphanumericCharset is a string listing all the characters that can be used in an alphanumeric string in QR codes.
	alphanumericCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"
)

// QrSegment is the representation of a segment of a QR code.
type QrSegment struct {
	mode     Mode       // The mode of the QR segment (e.g., Numeric, Alphanumeric, etc.)
	numChars int        // Number of characters in the segment
	data     *BitBuffer // The actual binary data represented by this QR segment
}

// newQrSegment function creates a new QR segment with the given mode, number of characters, and data.
func newQrSegment(mode Mode, numCh int, data *BitBuffer) (*QrSegment, error) {
	if numCh < 0 {
		return nil, errors.New("invalid value")
	}
	return &QrSegment{
		mode:     mode,
		numChars: numCh,
		data:     data.clone(), // Use a cloned copy of the data to prevent modifications
	}, nil
}

// getData method clones and returns the BitBuffer data of the QR segment.
func (q *QrSegment) getData() *BitBuffer {
	return q.data.clone()
}

// MakeBytes converts a byte slice into a QR segment in Byte mode.
// It returns an error if the input data is nil.
func MakeBytes(data []byte) (*QrSegment, error) {
	if data == nil {
		return nil, errors.New("data is nil")
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
		return nil, errors.New("string contains non-numeric characters")
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

// isNumeric function takes a string as input and returns a boolean indicating whether the string is numeric.
// It uses the MatchString method on the numericRegex to check the input string.
func isNumeric(numb string) bool {
	return numericRegex.MatchString(numb)
}

// isAlphanumeric function takes a string as input and returns a boolean indicating whether the string is alphanumeric.
// It uses the MatchString method on the alphanumericRegex to check the input string.
func isAlphanumeric(text string) bool {
	return alphanumericRegex.MatchString(text)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// MakeAlphanumeric converts a string into a QR code segment in Alphanumeric mode
// It returns an error if the string contains non-alphanumeric characters.
func MakeAlphanumeric(text string) (*QrSegment, error) {
	if !isAlphanumeric(text) {
		return nil, errors.New("string contains unencodable characters in alphanumeric mode")
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
		return nil, errors.New("ECI assignment value out of range")
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
		return nil, errors.New("ECI assignment value out of range")
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
