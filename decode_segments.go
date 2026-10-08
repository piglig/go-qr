package qr

import (
	"fmt"
	"unicode/utf8"
)

// bitReader reads big-endian bits out of the corrected data codewords.
type bitReader struct {
	data []byte
	pos  int // bit position
}

func (r *bitReader) remaining() int { return len(r.data)*8 - r.pos }

// read returns the next n bits as an int (n <= 32), or false if exhausted.
func (r *bitReader) read(n int) (int, bool) {
	if n > r.remaining() {
		return 0, false
	}
	v := 0
	for i := 0; i < n; i++ {
		v = v<<1 | int(r.data[r.pos>>3]>>uint(7-r.pos&7))&1
		r.pos++
	}
	return v, true
}

// noECI marks a segment that is not governed by any ECI designator.
const noECI = -1

// Mode indicators this decoder recognizes but does not support.
const (
	modeFNC1Second = 0x9 // FNC1 in second position (AIM application identifier)
	modeHanzi      = 0xD // GB/T 18284 Chinese mode
)

// bitstream is the parsed content of a symbol's data codewords.
type bitstream struct {
	text       string
	segs       []DecodedSegment
	structured *StructuredAppend
	gs1        bool
}

// parseBitstream walks the segment structure of the corrected data codewords
// (the reverse of encodeSegments) and returns the decoded text, segments and
// symbol-level indicators.
//
// Byte segments are interpreted by the ECI in effect: UTF-8 (26),
// ISO-8859-1 (1, 3), Shift_JIS (20) or ASCII (27, 170). Without an ECI, a
// segment that is valid UTF-8 is read as UTF-8, one that reads as Japanese
// Shift_JIS as Shift_JIS (see looksShiftJIS), and anything else as
// ISO-8859-1, which matches what common encoders emit. In a GS1 symbol, '%'
// in alphanumeric segments stands for the GS separator and "%%" for '%'.
func parseBitstream(data []byte, ver int) (bitstream, error) {
	r := &bitReader{data: data}
	var out bitstream
	var text []byte
	eci := noECI

	for r.remaining() >= 4 {
		bits, _ := r.read(4)
		mode := Mode(bits)
		switch mode {
		case 0: // terminator
			out.text = string(text)
			return out, nil
		case ModeECI:
			v, err := readECI(r)
			if err != nil {
				return bitstream{}, err
			}
			eci = v
			out.segs = append(out.segs, DecodedSegment{Mode: ModeECI, ECI: v})
			continue
		case ModeStructuredAppend:
			v, ok := r.read(16)
			if !ok {
				return bitstream{}, fmt.Errorf("%w: truncated structured append header", ErrDecodeFailed)
			}
			out.structured = &StructuredAppend{Index: v >> 12, Total: (v>>8)&0xF + 1, Parity: byte(v)}
			out.segs = append(out.segs, DecodedSegment{Mode: ModeStructuredAppend, ECI: eci})
			continue
		case ModeFNC1:
			out.gs1 = true
			out.segs = append(out.segs, DecodedSegment{Mode: ModeFNC1, ECI: eci})
			continue
		case ModeNumeric, ModeAlphanumeric, ModeByte, ModeKanji:
		case modeFNC1Second:
			return bitstream{}, fmt.Errorf("%w: FNC1 in second position", ErrUnsupported)
		case modeHanzi:
			return bitstream{}, fmt.Errorf("%w: Hanzi mode", ErrUnsupported)
		default:
			return bitstream{}, fmt.Errorf("%w: unknown mode %#x", ErrDecodeFailed, bits)
		}

		count, ok := r.read(mode.charCountBits(ver))
		if !ok {
			return bitstream{}, fmt.Errorf("%w: truncated character count", ErrDecodeFailed)
		}

		seg := DecodedSegment{Mode: mode, NumChars: count, ECI: eci}
		var err error
		switch mode {
		case ModeNumeric:
			seg.Data, err = readNumeric(r, count)
			text = append(text, seg.Data...)
		case ModeAlphanumeric:
			seg.Data, err = readAlphanumeric(r, count)
			if out.gs1 {
				text = appendGS1Alphanumeric(text, seg.Data)
			} else {
				text = append(text, seg.Data...)
			}
		case ModeByte:
			if seg.Data, err = readBytes(r, count); err == nil {
				text, err = appendCharset(text, seg.Data, eci)
			}
		case ModeKanji:
			seg.Data, text, err = readKanji(r, count, text)
		}
		if err != nil {
			return bitstream{}, err
		}
		out.segs = append(out.segs, seg)
	}
	out.text = string(text)
	return out, nil
}

// appendGS1Alphanumeric appends alphanumeric data from a GS1 symbol,
// turning '%' into the GS separator and "%%" into '%'.
func appendGS1Alphanumeric(text, data []byte) []byte {
	for i := 0; i < len(data); i++ {
		switch {
		case data[i] != '%':
			text = append(text, data[i])
		case i+1 < len(data) && data[i+1] == '%':
			text = append(text, '%')
			i++
		default:
			text = append(text, gs1Separator)
		}
	}
	return text
}

// readECI reads an ECI assignment number in its 1-, 2- or 3-byte form.
func readECI(r *bitReader) (int, error) {
	truncated := fmt.Errorf("%w: truncated ECI", ErrDecodeFailed)
	first, ok := r.read(8)
	if !ok {
		return 0, truncated
	}
	switch {
	case first&0x80 == 0:
		return first, nil
	case first&0xC0 == 0x80:
		rest, ok := r.read(8)
		if !ok {
			return 0, truncated
		}
		return (first&0x3F)<<8 | rest, nil
	case first&0xE0 == 0xC0:
		rest, ok := r.read(16)
		if !ok {
			return 0, truncated
		}
		return (first&0x1F)<<16 | rest, nil
	}
	return 0, fmt.Errorf("%w: invalid ECI designator %#x", ErrDecodeFailed, first)
}

func readNumeric(r *bitReader, count int) ([]byte, error) {
	out := make([]byte, 0, count)
	for count > 0 {
		n := min(count, 3)
		v, ok := r.read(n*3 + 1)
		if !ok {
			return nil, fmt.Errorf("%w: truncated numeric segment", ErrDecodeFailed)
		}
		if v >= [4]int{1, 10, 100, 1000}[n] {
			return nil, fmt.Errorf("%w: numeric group %d exceeds %d digits", ErrDecodeFailed, v, n)
		}
		start := len(out)
		out = append(out, "000"[:n]...)
		for i := start + n - 1; i >= start; i-- {
			out[i] = byte('0' + v%10)
			v /= 10
		}
		count -= n
	}
	return out, nil
}

func readAlphanumeric(r *bitReader, count int) ([]byte, error) {
	out := make([]byte, 0, count)
	for ; count >= 2; count -= 2 {
		v, ok := r.read(11)
		if !ok {
			return nil, fmt.Errorf("%w: truncated alphanumeric segment", ErrDecodeFailed)
		}
		if v >= 45*45 {
			return nil, fmt.Errorf("%w: alphanumeric pair value %d out of range", ErrDecodeFailed, v)
		}
		out = append(out, alphanumericCharset[v/45], alphanumericCharset[v%45])
	}
	if count == 1 {
		v, ok := r.read(6)
		if !ok {
			return nil, fmt.Errorf("%w: truncated alphanumeric segment", ErrDecodeFailed)
		}
		if v >= 45 {
			return nil, fmt.Errorf("%w: alphanumeric value %d out of range", ErrDecodeFailed, v)
		}
		out = append(out, alphanumericCharset[v])
	}
	return out, nil
}

func readBytes(r *bitReader, count int) ([]byte, error) {
	if count*8 > r.remaining() {
		return nil, fmt.Errorf("%w: truncated byte segment", ErrDecodeFailed)
	}
	out := make([]byte, count)
	for i := range out {
		v, _ := r.read(8)
		out[i] = byte(v)
	}
	return out, nil
}

// readKanji reads count 13-bit Kanji values. It returns the segment's
// Shift_JIS bytes and text with the characters appended as UTF-8; values
// without a Unicode mapping become U+FFFD.
func readKanji(r *bitReader, count int, text []byte) (sjis, _ []byte, err error) {
	if count*13 > r.remaining() {
		return nil, nil, fmt.Errorf("%w: truncated kanji segment", ErrDecodeFailed)
	}
	sjis = make([]byte, 0, 2*count)
	for i := 0; i < count; i++ {
		v, _ := r.read(13)
		b1, b2 := kanjiToShiftJIS(v)
		sjis = append(sjis, b1, b2)
		c, ok := kanjiRune(v)
		if !ok {
			c = utf8.RuneError
		}
		text = utf8.AppendRune(text, c)
	}
	return sjis, text, nil
}

// appendCharset appends the byte-mode payload b to text as UTF-8, decoding
// it according to the ECI assignment in effect.
func appendCharset(text, b []byte, eci int) ([]byte, error) {
	switch eci {
	case noECI:
		switch {
		case utf8.Valid(b):
			return append(text, b...), nil
		case looksShiftJIS(b):
			return appendShiftJIS(text, b), nil
		}
		return appendLatin1(text, b), nil
	case 26, 27, 170: // UTF-8; US-ASCII is a subset
		return append(text, b...), nil
	case 1, 3: // ISO-8859-1
		return appendLatin1(text, b), nil
	case 20: // Shift_JIS
		return appendShiftJIS(text, b), nil
	}
	return nil, fmt.Errorf("%w: ECI %d character set", ErrUnsupported, eci)
}

// looksShiftJIS reports whether a byte segment without an ECI, which is not
// UTF-8, is more likely Japanese Shift_JIS than ISO-8859-1. Japanese
// encoders commonly write Shift_JIS without declaring it. The bytes must
// form valid Shift_JIS, every double-byte character in the QR Kanji range,
// and either contain a run of at least three half-width katakana or
// double-byte characters, which Latin text does not produce, or contain
// bytes 0x80 to 0x9F, which are control characters in ISO-8859-1 and do not
// occur in text. (This follows the idea of ZXing's guessEncoding.)
func looksShiftJIS(b []byte) bool {
	var (
		run, longest int  // current and longest run of katakana or double-byte characters
		c1Control    bool // a byte in 0x80-0x9F
	)
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c < 0x80:
			run = 0
			continue
		case 0xA1 <= c && c <= 0xDF: // half-width katakana
		case i+1 < len(b):
			v, ok := shiftJISToKanji(c, b[i+1])
			if !ok {
				return false
			}
			if _, ok := kanjiRune(v); !ok {
				return false
			}
			c1Control = c1Control || c <= 0x9F || b[i+1] >= 0x80 && b[i+1] <= 0x9F
			i++
		default:
			return false // a lead byte at the end, or 0x80, 0xA0, 0xE0-0xFF alone
		}
		run++
		longest = max(longest, run)
	}
	return longest >= 3 || c1Control
}

func appendLatin1(text, b []byte) []byte {
	for _, c := range b {
		text = utf8.AppendRune(text, rune(c))
	}
	return text
}

// appendShiftJIS decodes Shift_JIS: ASCII, half-width katakana, and the
// double-byte range covered by the QR Kanji table. Anything else becomes
// U+FFFD.
func appendShiftJIS(text, b []byte) []byte {
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c < 0x80:
			text = append(text, c)
		case 0xA1 <= c && c <= 0xDF:
			text = utf8.AppendRune(text, 0xFF61+rune(c-0xA1))
		case i+1 < len(b):
			r := utf8.RuneError
			if v, ok := shiftJISToKanji(c, b[i+1]); ok {
				if k, ok := kanjiRune(v); ok {
					r = k
				}
			}
			text = utf8.AppendRune(text, r)
			i++
		default:
			text = utf8.AppendRune(text, utf8.RuneError)
		}
	}
	return text
}
