package qr

import (
	"fmt"
	"unicode/utf8"

	"github.com/piglig/go-qr/v2/internal/charset"
	"github.com/piglig/go-qr/v2/internal/symbol"
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
		return simpleSegments(text, c.gs1), nil
	}
	runes := []rune(text)
	if len(runes) > maxOptimalChars {
		return nil, fmt.Errorf("%w: %d characters exceed the maximum of %d", ErrDataTooLong, len(runes), maxOptimalChars)
	}

	charBits := minCharBits(runes, c.gs1)
	var segs []Segment
	for ver := c.minVer; ; ver++ {
		if ver == c.minVer || ver == 10 || ver == 27 {
			// Skip versions with this character count width when even the
			// cheapest mode for every character, in one segment, overflows
			// the largest of them; they are never skipped up to maxVer, so
			// errors come from the same segments as without the skip.
			next := 41
			if ver < 10 {
				next = 10
			} else if ver < 27 {
				next = 27
			}
			if next <= c.maxVer && minSegmentBits(charBits, ver)+extraBits > symbol.NumDataCodewords(next-1, symbol.Level(c.ecc))*8 {
				ver = next - 1
				continue
			}
			segs = splitSegments(text, runes, charModes(runes, ver, c.gs1), c.gs1)
		}
		capacityBits := symbol.NumDataCodewords(ver, symbol.Level(c.ecc)) * 8
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

// minCharBits returns a lower bound, in sixths of a bit, on the data bits of
// runes in any segmentation: each character in its cheapest mode, as
// charModes costs them.
func minCharBits(runes []rune, gs1 bool) int {
	total := 0
	for i, r := range runes {
		cost := utf8.RuneLen(r) * 8 * 6
		switch {
		case r < utf8.RuneSelf && isNumericByte(byte(r)):
			cost = 20
		case r < utf8.RuneSelf && isAlphanumericByte(byte(r)) && !(gs1 && r == '%'),
			gs1 && r == gs1Separator && gs1AlnumSeparator(runes, i):
			cost = 33
		default:
			if _, ok := charset.KanjiValue(r); ok {
				cost = min(cost, 78)
			}
		}
		total += cost
	}
	return total
}

// minSegmentBits returns a lower bound on the bits of any segmentation of
// characters whose data costs at least charBits sixths, at version ver: the
// data and the cheapest segment header.
func minSegmentBits(charBits, ver int) int {
	head := 4 + ModeByte.charCountBits(ver)
	for _, m := range optimalModes {
		head = min(head, 4+m.charCountBits(ver))
	}
	return (charBits+5)/6 + head
}

// optimalModes are the modes considered by charModes, in DP column order.
var optimalModes = [4]Mode{ModeByte, ModeAlphanumeric, ModeNumeric, ModeKanji}

// charModes returns the mode of every character in the cheapest encoding of
// runes at version ver. It is a dynamic program over (character, mode) pairs;
// costs are in sixths of a bit so that alphanumeric (5.5 bits/char) and
// numeric (3.33 bits/char) stay integral. With gs1, the GS separator is
// alphanumeric ('%') and '%' costs two alphanumeric characters ("%%").
func charModes(runes []rune, ver int, gs1 bool) []Mode {
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
		switch {
		case gs1 && r == '%':
			cur[1] = prev[1] + 66
			step[1] = ModeAlphanumeric
		case r < utf8.RuneSelf && isAlphanumericByte(byte(r)), gs1 && r == gs1Separator && gs1AlnumSeparator(runes, i):
			cur[1] = prev[1] + 33
			step[1] = ModeAlphanumeric
		}
		if r < utf8.RuneSelf && isNumericByte(byte(r)) {
			cur[2] = prev[2] + 20
			step[2] = ModeNumeric
		}
		if _, ok := charset.KanjiValue(r); ok {
			cur[3] = prev[3] + 78
			step[3] = ModeKanji
		}

		// Switching from mode k to mode j after this character costs the
		// partial bit rounded up plus the new segment header. The cheapest
		// mode to switch from is the same for every j: the first with the
		// least rounded cost. Staying in mode j wins ties.
		src, srcCost := 0, (cur[0]+5)/6*6 // byte mode is always possible
		for k := 1; k < n; k++ {
			if c := (cur[k] + 5) / 6 * 6; step[k] != 0 && c < srcCost {
				src, srcCost = k, c
			}
		}
		for j := 0; j < n; j++ {
			if cost := srcCost + head[j]; step[j] == 0 || cost < cur[j] {
				cur[j] = cost
				step[j] = optimalModes[src]
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
func splitSegments(text string, runes []rune, modes []Mode, gs1 bool) []Segment {
	var segs []Segment
	start, pos := 0, 0 // byte offsets into text
	for i, r := range runes {
		pos += utf8.RuneLen(r)
		if i+1 < len(runes) && modes[i+1] == modes[i] {
			continue
		}
		segs = append(segs, makeSegment(modes[i], text[start:pos], gs1))
		start = pos
	}
	return segs
}

// makeSegment builds a segment of mode m for s, which charModes has already
// verified to be encodable in that mode.
func makeSegment(m Mode, s string, gs1 bool) Segment {
	var seg Segment
	var err error
	switch m {
	case ModeNumeric:
		seg, err = NumericSegment(s)
	case ModeAlphanumeric:
		if gs1 {
			s = gs1Alphanumeric(s)
		}
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
