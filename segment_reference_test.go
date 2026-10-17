package qr

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/piglig/go-qr/v2/internal/symbol"
)

// The segmentation of v2.6, kept as the reference the faster one must
// match exactly.

// referenceOptimalSegments is optimalSegments up to v2.6.
// optimalSegments splits text into the sequence of segments with the fewest
// total bits, switching between numeric, alphanumeric, byte and Kanji modes.
// The optimal split depends on the width of the character count indicators,
// which changes at versions 10 and 27, so the split is recomputed at those
// boundaries while searching for the smallest version that fits. extraBits is
// the size of any segments that will be prepended.
func referenceOptimalSegments(text string, c encodeConfig, extraBits int) ([]Segment, error) {
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

	var segs []Segment
	for ver := c.minVer; ; ver++ {
		if ver == c.minVer || ver == 10 || ver == 27 {
			segs = splitSegments(text, runes, referenceCharModes(runes, ver, c.gs1), c.gs1)
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

// referenceCharModes is charModes up to v2.6.
// charModes returns the mode of every character in the cheapest encoding of
// runes at version ver. It is a dynamic program over (character, mode) pairs;
// costs are in sixths of a bit so that alphanumeric (5.5 bits/char) and
// numeric (3.33 bits/char) stay integral. With gs1, the GS separator is
// alphanumeric ('%') and '%' costs two alphanumeric characters ("%%").
func referenceCharModes(runes []rune, ver int, gs1 bool) []Mode {
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

// segmentText builds a random text of runs drawn from character classes
// that favor mode switches: digits, alphanumerics, lowercase, the GS1
// separator and '%', Kanji, and other multi-byte characters.
func segmentText(rng *rand.Rand, maxLen int) string {
	classes := [][]rune{
		[]rune("0123456789"),
		[]rune("ABCXYZ $*+-./:"),
		append([]rune("abcxyz!?"), 0x5C),
		{gs1Separator, '%'},
		[]rune("漢字日本アイαβ"),
		[]rune("é€😀ü"),
	}
	n := rng.Intn(maxLen + 1)
	out := make([]rune, 0, n)
	for len(out) < n {
		cls := classes[rng.Intn(len(classes))]
		for k := 1 + rng.Intn(12); k > 0 && len(out) < n; k-- {
			out = append(out, cls[rng.Intn(len(cls))])
		}
	}
	return string(out)
}

func sameSegments(a, b []Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].mode != b[i].mode || a[i].numChars != b[i].numChars || a[i].data.len() != b[i].data.len() {
			return false
		}
		for k := 0; k < a[i].data.len(); k++ {
			if a[i].data.getBit(k) != b[i].data.getBit(k) {
				return false
			}
		}
	}
	return true
}

func TestCharModesMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 3000; i++ {
		runes := []rune(segmentText(rng, 300))
		gs1 := rng.Intn(3) == 0
		for _, ver := range []int{1, 10, 27} {
			got, want := charModes(runes, ver, gs1), referenceCharModes(runes, ver, gs1)
			for k := range want {
				if got[k] != want[k] {
					t.Fatalf("%q gs1=%v v%d: mode of character %d is %v, want %v", string(runes), gs1, ver, k, got[k], want[k])
				}
			}
		}
	}
}

func TestOptimalSegmentsMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	skipped := 0
	for i := 0; i < 1500; i++ {
		text := segmentText(rng, 3000)
		minV := 1 + rng.Intn(40)
		maxV := minV + rng.Intn(41-minV)
		opts := []EncodeOption{WithVersionRange(minV, maxV), WithECC(ECC(rng.Intn(4)))}
		if rng.Intn(3) == 0 {
			opts = append(opts, WithGS1())
		}
		c, err := newEncodeConfig(opts)
		if err != nil {
			t.Fatal(err)
		}
		extra := []int{0, 20, 36}[rng.Intn(3)]
		got, gotErr := optimalSegments(text, c, extra)
		want, wantErr := referenceOptimalSegments(text, c, extra)
		if (gotErr == nil) != (wantErr == nil) || gotErr != nil && gotErr.Error() != wantErr.Error() {
			t.Fatalf("%d chars, v%d-%d: error %v, want %v", len([]rune(text)), minV, maxV, gotErr, wantErr)
		}
		if !sameSegments(got, want) {
			t.Fatalf("%d chars, v%d-%d: segments differ", len([]rune(text)), minV, maxV)
		}
		if minV < 10 && maxV >= 10 && minSegmentBits(minCharBits([]rune(text), c.gs1), minV)+extra > symbol.NumDataCodewords(9, symbol.Level(c.ecc))*8 {
			skipped++
		}
	}
	if skipped < 100 {
		t.Errorf("only %d texts skipped a range of versions", skipped)
	}
}

func TestMinSegmentBitsIsALowerBound(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 2000; i++ {
		text := segmentText(rng, 500)
		if text == "" {
			continue
		}
		runes := []rune(text)
		gs1 := rng.Intn(3) == 0
		bound := minCharBits(runes, gs1)
		for _, ver := range []int{1, 10, 27} {
			segs := splitSegments(text, runes, referenceCharModes(runes, ver, gs1), gs1)
			if bits := totalBits(segs, ver); bits != -1 && minSegmentBits(bound, ver) > bits {
				t.Fatalf("%q gs1=%v v%d: bound %d exceeds %d bits", text, gs1, ver, minSegmentBits(bound, ver), bits)
			}
		}
	}
	// Some characters of the Kanji table, such as U+005C, are cheaper in
	// byte mode; the bound takes the cheaper.
	for r := rune(0); r < 0x10000; r++ {
		if _, ok := kanjiValue(r); ok {
			if got, want := minCharBits([]rune{r}, false), min(utf8.RuneLen(r)*8*6, 78); got != want && !(r < utf8.RuneSelf && isAlphanumericByte(byte(r))) {
				t.Fatalf("%U: bound %d, want %d", r, got, want)
			}
		}
	}
}

func TestAlphanumericValue(t *testing.T) {
	for c := 0; c < 256; c++ {
		if got, want := int(alphanumericValue[c]), strings.IndexByte(alphanumericCharset, byte(c)); got != want {
			t.Fatalf("alphanumericValue[%#x] = %d, want %d", c, got, want)
		}
	}
}
