package qr

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Mode is a QR segment mode. Its value is the 4-bit mode indicator written
// into the bitstream.
type Mode uint8

// Segment modes supported by this package (ISO/IEC 18004 §7.4).
const (
	ModeNumeric          Mode = 0x1 // decimal digits 0-9
	ModeAlphanumeric     Mode = 0x2 // 0-9, A-Z, space and $%*+-./:
	ModeStructuredAppend Mode = 0x3 // position of the symbol in a sequence
	ModeByte             Mode = 0x4 // arbitrary bytes
	ModeFNC1             Mode = 0x5 // FNC1 in first position: GS1 data
	ModeECI              Mode = 0x7 // extended channel interpretation designator
	ModeKanji            Mode = 0x8 // Shift_JIS double-byte characters
)

// String returns the mode name, such as "numeric".
func (m Mode) String() string {
	switch m {
	case ModeNumeric:
		return "numeric"
	case ModeAlphanumeric:
		return "alphanumeric"
	case ModeStructuredAppend:
		return "structured append"
	case ModeByte:
		return "byte"
	case ModeFNC1:
		return "fnc1"
	case ModeECI:
		return "eci"
	case ModeKanji:
		return "kanji"
	}
	return fmt.Sprintf("Mode(%#x)", uint8(m))
}

// charCountBits returns the width of the character count indicator for the
// mode at the given version (ISO/IEC 18004 Table 3).
func (m Mode) charCountBits(ver int) int {
	r := (ver + 7) / 17 // 0 for versions 1-9, 1 for 10-26, 2 for 27-40
	switch m {
	case ModeNumeric:
		return [3]int{10, 12, 14}[r]
	case ModeAlphanumeric:
		return [3]int{9, 11, 13}[r]
	case ModeByte:
		return [3]int{8, 16, 16}[r]
	case ModeKanji:
		return [3]int{8, 10, 12}[r]
	}
	return 0
}

// alphanumericCharset lists every character encodable in alphanumeric mode; the
// index of a character is also its alphanumeric value.
const alphanumericCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

// Segment is one run of data in a single mode. Most callers never build
// segments directly: Encode picks them automatically. Use the constructors
// below with EncodeSegments to control the encoding exactly.
type Segment struct {
	mode     Mode
	numChars int
	data     bitBuffer
}

// Mode returns the segment mode.
func (s Segment) Mode() Mode { return s.mode }

// NumChars returns the value of the segment's character count indicator: the
// number of digits, characters, bytes or Kanji. It is 0 for ECI segments.
func (s Segment) NumChars() int { return s.numChars }

// NumericSegment returns a numeric-mode segment for a string of ASCII digits.
func NumericSegment(digits string) (Segment, error) {
	if !isNumeric(digits) {
		return Segment{}, fmt.Errorf("%w: %q is not numeric", ErrUnencodableChar, digits)
	}
	var bb bitBuffer
	for i := 0; i < len(digits); i += 3 {
		n := min(len(digits)-i, 3)
		val := 0
		for _, d := range digits[i : i+n] {
			val = val*10 + int(d-'0')
		}
		bb.appendBits(val, n*3+1)
	}
	return Segment{mode: ModeNumeric, numChars: len(digits), data: bb}, nil
}

// AlphanumericSegment returns an alphanumeric-mode segment. text may contain
// only 0-9, A-Z (uppercase), space and $%*+-./:.
func AlphanumericSegment(text string) (Segment, error) {
	if !isAlphanumeric(text) {
		return Segment{}, fmt.Errorf("%w: %q is not alphanumeric", ErrUnencodableChar, text)
	}
	var bb bitBuffer
	i := 0
	for ; i+1 < len(text); i += 2 {
		val := strings.IndexByte(alphanumericCharset, text[i])*45 +
			strings.IndexByte(alphanumericCharset, text[i+1])
		bb.appendBits(val, 11)
	}
	if i < len(text) {
		bb.appendBits(strings.IndexByte(alphanumericCharset, text[i]), 6)
	}
	return Segment{mode: ModeAlphanumeric, numChars: len(text), data: bb}, nil
}

// BytesSegment returns a byte-mode segment holding data verbatim.
func BytesSegment(data []byte) Segment {
	var bb bitBuffer
	bb.grow(len(data) * 8)
	for _, b := range data {
		bb.appendBits(int(b), 8)
	}
	return Segment{mode: ModeByte, numChars: len(data), data: bb}
}

// KanjiSegment returns a Kanji-mode segment. Every character of text must be
// in the Shift_JIS double-byte range covered by QR Kanji mode.
func KanjiSegment(text string) (Segment, error) {
	var bb bitBuffer
	n := 0
	for _, r := range text {
		v, ok := kanjiValue(r)
		if !ok {
			return Segment{}, fmt.Errorf("%w: %q is not encodable in kanji mode", ErrUnencodableChar, r)
		}
		bb.appendBits(v, 13)
		n++
	}
	return Segment{mode: ModeKanji, numChars: n, data: bb}, nil
}

// ECISegment returns an ECI designator segment. It tells the reader how to
// interpret the bytes of following byte-mode segments; for example 26 is
// UTF-8 and 3 is ISO-8859-1. assignment must be in [0, 999999].
func ECISegment(assignment int) (Segment, error) {
	var bb bitBuffer
	switch {
	case assignment < 0 || assignment > 999999:
		return Segment{}, fmt.Errorf("%w: ECI assignment %d out of range", ErrInvalidArgument, assignment)
	case assignment < 1<<7:
		bb.appendBits(assignment, 8)
	case assignment < 1<<14:
		bb.appendBits(0b10, 2)
		bb.appendBits(assignment, 14)
	default:
		bb.appendBits(0b110, 3)
		bb.appendBits(assignment, 21)
	}
	return Segment{mode: ModeECI, data: bb}, nil
}

// eciUTF8 is the ECI assignment number for UTF-8.
const eciUTF8 = 26

// structuredAppendSegment returns the header that marks a symbol as number
// index (0-based) of total symbols carrying one message whose bytes XOR to
// parity (ISO/IEC 18004 §8).
func structuredAppendSegment(index, total int, parity byte) Segment {
	var bb bitBuffer
	bb.appendBits(index, 4)
	bb.appendBits(total-1, 4)
	bb.appendBits(int(parity), 8)
	return Segment{mode: ModeStructuredAppend, data: bb}
}

// fnc1Segment returns the FNC1-in-first-position indicator that marks the
// data as a GS1 element string.
func fnc1Segment() Segment { return Segment{mode: ModeFNC1} }

// gs1Separator is the ASCII GS character that separates variable-length GS1
// element strings. Alphanumeric segments write it as '%' and a literal '%'
// as "%%" (ISO/IEC 18004 §7.4.8.2); other modes carry it as is.
const gs1Separator = 0x1D

// gs1Alphanumeric rewrites s for an alphanumeric segment in a GS1 symbol.
func gs1Alphanumeric(s string) string {
	return strings.NewReplacer("%", "%%", string(rune(gs1Separator)), "%").Replace(s)
}

// isNumeric reports whether s is non-empty and contains only ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isNumericByte(s[i]) {
			return false
		}
	}
	return true
}

// isAlphanumeric reports whether every byte of s is encodable in alphanumeric
// mode.
func isAlphanumeric(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isAlphanumericByte(s[i]) {
			return false
		}
	}
	return true
}

func isNumericByte(c byte) bool { return '0' <= c && c <= '9' }

func isAlphanumericByte(c byte) bool { return strings.IndexByte(alphanumericCharset, c) >= 0 }

// isASCII reports whether s contains only 7-bit characters.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// simpleSegments encodes text as a single segment in the most compact mode
// that can hold all of it: numeric, alphanumeric or byte. With gs1, the GS
// separator counts as alphanumeric.
func simpleSegments(text string, gs1 bool) []Segment {
	switch {
	case text == "":
		return nil
	case isNumeric(text):
		s, _ := NumericSegment(text)
		return []Segment{s}
	case gs1 && isAlphanumeric(strings.ReplaceAll(text, string(rune(gs1Separator)), "")):
		s, _ := AlphanumericSegment(gs1Alphanumeric(text))
		return []Segment{s}
	case !gs1 && isAlphanumeric(text):
		s, _ := AlphanumericSegment(text)
		return []Segment{s}
	}
	return []Segment{BytesSegment([]byte(text))}
}

// totalBits returns the number of bits needed to encode segs at version ver,
// or -1 if a segment's character count does not fit its count indicator.
func totalBits(segs []Segment, ver int) int {
	total := 0
	for _, s := range segs {
		ccBits := s.mode.charCountBits(ver)
		if ccBits > 0 && s.numChars >= 1<<ccBits {
			return -1
		}
		total += 4 + ccBits + s.data.len()
	}
	return total
}
