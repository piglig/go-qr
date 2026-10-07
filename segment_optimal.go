package qr

import (
	"fmt"
	"unicode/utf8"
)

// maxOptimalChars bounds the input to the optimal segmenter; no QR Code holds
// more than 7089 characters (numeric mode, version 40-L).
const maxOptimalChars = 7089

// optimalSegments splits text into the sequence of segments with the fewest
// total bits, switching between numeric, alphanumeric, byte and Kanji modes.
// The optimal split depends on the width of the character count indicators,
// which changes at versions 10 and 27, so the split is recomputed at those
// boundaries while searching for the smallest version that fits. extraBits is
// the size of any segments that will be prepended.
func optimalSegments(text string, c encodeConfig, extraBits int) ([]Segment, error) {
	if text == "" {
		return nil, nil
	}
	if !utf8.ValidString(text) {
		// Invalid UTF-8 can only be carried byte for byte.
		return simpleSegments(text), nil
	}
	runes := []rune(text)
	if len(runes) > maxOptimalChars {
		return nil, fmt.Errorf("%w: %d characters exceed the maximum of %d", ErrDataTooLong, len(runes), maxOptimalChars)
	}

	var segs []Segment
	for ver := c.minVer; ; ver++ {
		if ver == c.minVer || ver == 10 || ver == 27 {
			segs = splitSegments(text, runes, charModes(runes, ver))
		}
		capacityBits := numDataCodewords(ver, c.ecc) * 8
		usedBits := totalBits(segs, ver)
		if usedBits != -1 && usedBits+extraBits <= capacityBits {
			return segs, nil
		}
		if ver >= c.maxVer {
			// Report the overflow with the same error encodeSegments would.
			_, _, err := fitVersion(segs, c.ecc, ver, ver)
			if err == nil {
				err = fmt.Errorf("%w: data does not fit in versions %d..%d", ErrDataTooLong, c.minVer, c.maxVer)
			}
			return nil, err
		}
	}
}

// optimalModes are the modes considered by charModes, in DP column order.
var optimalModes = [4]Mode{ModeByte, ModeAlphanumeric, ModeNumeric, ModeKanji}

// charModes returns the mode of every character in the cheapest encoding of
// runes at version ver. It is a dynamic program over (character, mode) pairs;
// costs are in sixths of a bit so that alphanumeric (5.5 bits/char) and
// numeric (3.33 bits/char) stay integral.
func charModes(runes []rune, ver int) []Mode {
	const n = len(optimalModes)
	var head [n]int // cost of starting a new segment in each mode
	for j, m := range optimalModes {
		head[j] = (4 + m.charCountBits(ver)) * 6
	}

	// from[i*n+j] is the mode of character i on the cheapest path whose
	// character i is in mode j; 0 means character i cannot be in mode j.
	from := make([]Mode, len(runes)*n)
	prev := head
	for i, r := range runes {
		var cur [n]int
		step := from[i*n : i*n+n]

		cur[0] = prev[0] + utf8.RuneLen(r)*8*6
		step[0] = ModeByte
		if r < utf8.RuneSelf && isAlphanumericByte(byte(r)) {
			cur[1] = prev[1] + 33
			step[1] = ModeAlphanumeric
		}
		if r < utf8.RuneSelf && isNumericByte(byte(r)) {
			cur[2] = prev[2] + 20
			step[2] = ModeNumeric
		}
		if _, ok := kanjiValue(r); ok {
			cur[3] = prev[3] + 78
			step[3] = ModeKanji
		}

		// Switching from mode k to mode j after this character costs the
		// partial bit rounded up plus the new segment header.
		for j := 0; j < n; j++ {
			for k := 0; k < n; k++ {
				cost := (cur[k]+5)/6*6 + head[j]
				if step[k] != 0 && (step[j] == 0 || cost < cur[j]) {
					cur[j] = cost
					step[j] = optimalModes[k]
				}
			}
		}
		prev = cur
	}

	best := 0
	for j := 1; j < n; j++ {
		if prev[j] < prev[best] {
			best = j
		}
	}

	modes := make([]Mode, len(runes))
	mode := optimalModes[best]
	for i := len(runes) - 1; i >= 0; i-- {
		mode = from[i*n+modeColumn(mode)]
		modes[i] = mode
	}
	return modes
}

// modeColumn returns the index of m in optimalModes.
func modeColumn(m Mode) int {
	switch m {
	case ModeAlphanumeric:
		return 1
	case ModeNumeric:
		return 2
	case ModeKanji:
		return 3
	}
	return 0
}

// splitSegments groups consecutive characters with the same mode into
// segments. text and runes are the same string; modes[i] is the mode of
// runes[i].
func splitSegments(text string, runes []rune, modes []Mode) []Segment {
	var segs []Segment
	start, pos := 0, 0 // byte offsets into text
	for i, r := range runes {
		pos += utf8.RuneLen(r)
		if i+1 < len(runes) && modes[i+1] == modes[i] {
			continue
		}
		segs = append(segs, makeSegment(modes[i], text[start:pos]))
		start = pos
	}
	return segs
}

// makeSegment builds a segment of mode m for s, which charModes has already
// verified to be encodable in that mode.
func makeSegment(m Mode, s string) Segment {
	var seg Segment
	var err error
	switch m {
	case ModeNumeric:
		seg, err = NumericSegment(s)
	case ModeAlphanumeric:
		seg, err = AlphanumericSegment(s)
	case ModeKanji:
		seg, err = KanjiSegment(s)
	default:
		seg = BytesSegment([]byte(s))
	}
	if err != nil {
		panic("qr: optimal segmentation chose an invalid mode: " + err.Error())
	}
	return seg
}
