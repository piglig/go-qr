package qr

import (
	"errors"
	"fmt"
)

// EncodeOption configures Encode, EncodeBytes and EncodeSegments.
type EncodeOption func(*encodeConfig)

type encodeConfig struct {
	ecc            ECC
	minVer, maxVer int
	mask           int  // -1 selects the lowest-penalty mask
	fixedMask      bool // WithMask was given
	boost          bool
	simple         bool
	utf8ECI        bool
	gs1            bool
}

func newEncodeConfig(opts []EncodeOption) (encodeConfig, error) {
	c := encodeConfig{ecc: ECCMedium, minVer: MinVersion, maxVer: MaxVersion, mask: -1, boost: true}
	for _, o := range opts {
		o(&c)
	}
	switch {
	case !c.ecc.valid():
		return c, fmt.Errorf("%w: unknown error correction level %d", ErrInvalidArgument, int8(c.ecc))
	case c.minVer < MinVersion || c.minVer > c.maxVer || c.maxVer > MaxVersion:
		return c, fmt.Errorf("%w: version range %d..%d", ErrInvalidVersion, c.minVer, c.maxVer)
	case c.fixedMask && (c.mask < 0 || c.mask > 7):
		return c, fmt.Errorf("%w: mask %d out of range 0..7", ErrInvalidArgument, c.mask)
	}
	return c, nil
}

// WithECC sets the minimum error correction level. The default is ECCMedium.
// Unless WithoutECCBoost is given, the level is raised as far as the data
// still fits in the chosen version.
func WithECC(level ECC) EncodeOption {
	return func(c *encodeConfig) { c.ecc = level }
}

// WithVersionRange restricts the symbol version to [min, max]. The smallest
// version in the range that holds the data is used. The default is
// [MinVersion, MaxVersion]; pass the same value twice to fix the version.
func WithVersionRange(min, max int) EncodeOption {
	return func(c *encodeConfig) { c.minVer, c.maxVer = min, max }
}

// WithMask forces mask pattern m (0-7) instead of choosing the pattern with
// the lowest ISO/IEC 18004 penalty score.
func WithMask(m int) EncodeOption {
	return func(c *encodeConfig) { c.mask, c.fixedMask = m, true }
}

// WithoutECCBoost keeps exactly the level given by WithECC instead of raising
// it when the chosen version has spare capacity.
func WithoutECCBoost() EncodeOption {
	return func(c *encodeConfig) { c.boost = false }
}

// WithSimpleSegmentation makes Encode use a single segment in the most
// compact mode that holds the whole text (numeric, alphanumeric or byte),
// instead of switching modes optimally within the text. It is faster, but can
// produce a larger symbol for mixed content. Ignored by EncodeBytes and
// EncodeSegments.
func WithSimpleSegmentation() EncodeOption {
	return func(c *encodeConfig) { c.simple = true }
}

// WithUTF8ECI prefixes the data with an ECI designator declaring UTF-8 when
// the text is not pure ASCII. Readers that follow ISO/IEC 18004 otherwise
// assume ISO-8859-1 for byte-mode data, although most phone scanners guess
// UTF-8 anyway. Ignored by EncodeSegments.
func WithUTF8ECI() EncodeOption {
	return func(c *encodeConfig) { c.utf8ECI = true }
}

// WithGS1 marks the data as a GS1 element string, such as
// "01095011015300031725010110ABC123", by writing the FNC1-in-first-position
// indicator. Separate a variable-length element from the next one with the
// ASCII GS character (0x1D); Encode turns it into the FNC1 code GS1 readers
// expect. Do not include the human-readable parentheses around application
// identifiers. Ignored by EncodeBytes and EncodeSegments.
func WithGS1() EncodeOption {
	return func(c *encodeConfig) { c.gs1 = true }
}

// Encode encodes text into a QR Code. By default it uses ECCMedium (boosted
// when there is room), the smallest fitting version, optimal mode switching
// between numeric, alphanumeric, byte and Kanji segments, and the
// lowest-penalty mask.
func Encode(text string, opts ...EncodeOption) (*Code, error) {
	c, err := newEncodeConfig(opts)
	if err != nil {
		return nil, err
	}
	segs, err := textSegments(text, c, nil)
	if err != nil {
		return nil, err
	}
	return encodeSegments(segs, c)
}

// textSegments returns the segments that encode text under c: the given
// header, then the FNC1 and ECI designators the options call for, then the
// data.
func textSegments(text string, c encodeConfig, header []Segment) ([]Segment, error) {
	prefix := append([]Segment(nil), header...)
	if c.gs1 {
		prefix = append(prefix, fnc1Segment())
	}
	if c.utf8ECI && !isASCII(text) {
		eci, _ := ECISegment(eciUTF8)
		prefix = append(prefix, eci)
	}
	if c.simple {
		return append(prefix, simpleSegments(text, c.gs1)...), nil
	}
	data, err := optimalSegments(text, c, totalBits(prefix, MinVersion))
	if err != nil {
		return nil, err
	}
	return append(prefix, data...), nil
}

// maxStructuredAppend is the largest number of symbols in a structured
// append sequence.
const maxStructuredAppend = 16

// EncodeStructured encodes text across the fewest symbols, up to 16, that
// each fit the options, linking them with structured append headers so a
// reader can reassemble the message (see JoinStructuredAppend). Restrict the
// size of each symbol with WithVersionRange. A text that fits in one symbol
// yields a single code without a header. The text is split between
// characters, so every symbol holds valid UTF-8.
func EncodeStructured(text string, opts ...EncodeOption) ([]*Code, error) {
	c, err := newEncodeConfig(opts)
	if err != nil {
		return nil, err
	}
	if code, err := Encode(text, opts...); err == nil {
		return []*Code{code}, nil
	} else if !errors.Is(err, ErrDataTooLong) {
		return nil, err
	}

	var parity byte
	for i := 0; i < len(text); i++ {
		parity ^= text[i]
	}
	runes := []rune(text)
	for n := 2; n <= min(maxStructuredAppend, len(runes)); n++ {
		codes, err := encodeParts(runes, n, parity, c)
		if err == nil {
			return codes, nil
		}
		if !errors.Is(err, ErrDataTooLong) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: text does not fit in %d symbols of versions %d..%d",
		ErrDataTooLong, maxStructuredAppend, c.minVer, c.maxVer)
}

// encodeParts encodes runes as n symbols holding nearly equal numbers of
// characters.
func encodeParts(runes []rune, n int, parity byte, c encodeConfig) ([]*Code, error) {
	codes := make([]*Code, n)
	for i := range codes {
		part := string(runes[i*len(runes)/n : (i+1)*len(runes)/n])
		segs, err := textSegments(part, c, []Segment{structuredAppendSegment(i, n, parity)})
		if err != nil {
			return nil, err
		}
		if codes[i], err = encodeSegments(segs, c); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// EncodeBytes encodes binary data in a single byte-mode segment.
func EncodeBytes(data []byte, opts ...EncodeOption) (*Code, error) {
	c, err := newEncodeConfig(opts)
	if err != nil {
		return nil, err
	}
	segs := []Segment{BytesSegment(data)}
	if c.utf8ECI && !isASCII(string(data)) {
		eci, _ := ECISegment(eciUTF8)
		segs = append([]Segment{eci}, segs...)
	}
	return encodeSegments(segs, c)
}

// EncodeSegments encodes the given segments as-is, in order.
func EncodeSegments(segs []Segment, opts ...EncodeOption) (*Code, error) {
	c, err := newEncodeConfig(opts)
	if err != nil {
		return nil, err
	}
	return encodeSegments(segs, c)
}

// encodeSegments picks the smallest version in range that holds segs, boosts
// the ECC level if allowed, builds the padded data codewords and lays out the
// symbol.
func encodeSegments(segs []Segment, c encodeConfig) (*Code, error) {
	ver, usedBits, err := fitVersion(segs, c.ecc, c.minVer, c.maxVer)
	if err != nil {
		return nil, err
	}

	ecc := c.ecc
	if c.boost {
		for _, e := range []ECC{ECCMedium, ECCQuartile, ECCHigh} {
			if e > ecc && usedBits <= numDataCodewords(ver, e)*8 {
				ecc = e
			}
		}
	}

	capacityBits := numDataCodewords(ver, ecc) * 8
	var bb bitBuffer
	bb.grow(capacityBits)
	for _, s := range segs {
		bb.appendBits(int(s.mode), 4)
		bb.appendBits(s.numChars, s.mode.charCountBits(ver))
		bb.appendBuffer(&s.data)
	}

	// Terminator, then pad to a byte boundary, then alternate pad bytes.
	bb.appendBits(0, min(4, capacityBits-bb.len()))
	bb.appendBits(0, (8-bb.len()%8)%8)
	for pad := 0xEC; bb.len() < capacityBits; pad ^= 0xEC ^ 0x11 {
		bb.appendBits(pad, 8)
	}
	return newCode(ver, ecc, bb.bytes(), c.mask)
}

// fitVersion returns the smallest version in [minVer, maxVer] whose data
// capacity at level ecc holds segs, along with the number of bits used.
func fitVersion(segs []Segment, ecc ECC, minVer, maxVer int) (ver, usedBits int, err error) {
	for ver = minVer; ; ver++ {
		capacityBits := numDataCodewords(ver, ecc) * 8
		usedBits = totalBits(segs, ver)
		if usedBits != -1 && usedBits <= capacityBits {
			return ver, usedBits, nil
		}
		if ver >= maxVer {
			if usedBits == -1 {
				return 0, 0, fmt.Errorf("%w: segment too long for versions %d..%d", ErrDataTooLong, minVer, maxVer)
			}
			return 0, 0, fmt.Errorf("%w: %d bits exceed the %d-bit capacity of version %d-%v",
				ErrDataTooLong, usedBits, capacityBits, ver, ecc)
		}
	}
}

// numDataCodewords returns the number of data (non-ECC) codewords in a symbol
// of the given version and ECC level.
func numDataCodewords(ver int, ecc ECC) int {
	return numRawDataModules(ver)/8 -
		int(eccCodeWordsPerBlock[ecc][ver])*int(numErrorCorrectionBlocks[ecc][ver])
}
