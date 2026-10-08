package qr

import "unicode/utf8"

// Character set inference for byte segments without an ECI.
//
// A byte segment says nothing about its character set unless an ECI
// precedes it, and encoders commonly omit one: Japanese ones write
// Shift_JIS, others ISO-8859-1 or, from Windows software, Windows-1252,
// which uses ISO-8859-1's C1 control codes for “quotes”, – dashes, € and
// the like. Valid UTF-8 is read as UTF-8, since
// multi-byte UTF-8 sequences do not arise by chance in either. For the rest,
// each candidate character set decodes the bytes, and the decoded text is
// scored by a model of how characters follow each other in text: a
// first-order Markov chain over character classes, with costs (negative log
// plausibilities, in rough bits) set from how the scripts are written.
// Accented Latin letters sit inside words of ASCII letters; katakana,
// hiragana and kanji run together; half-width katakana spell words rather
// than standing alone, as the Latin-1 symbols sharing their bytes do. The
// cheapest reading wins. Undeclared single-byte text is read as
// Windows-1252, a superset of ISO-8859-1 for text, as browsers do.
//
// All undeclared byte segments of a symbol come from one encoder, so they
// are scored together and read in one character set; a Kanji mode segment
// in the symbol marks a Japanese encoder.

// charset is a character set a byte segment can be read in.
type charset uint8

const (
	charsetUTF8 charset = iota
	charsetShiftJIS
	charsetLatin1 // Windows-1252, a superset of ISO-8859-1 for text
)

// charClass is a class of characters with similar roles in text.
type charClass uint8

const (
	clsStart      charClass = iota // before the first character
	clsLetter                      // ASCII letter
	clsDigit                       // ASCII digit
	clsSpace                       // space, tab, CR, LF
	clsPunct                       // other printable ASCII
	clsControl                     // other C0 controls, DEL, C1 controls
	clsAccented                    // Latin-1 letter: À-ÿ except × and ÷
	clsRareLetter                  // Windows-1252 letters: Œ œ Š š Ž ž Ÿ
	clsSymbol                      // Latin-1 symbol: no-break space, ¡-¿, ×, ÷; € ™ ƒ
	clsTypo                        // typographic punctuation: ‘ ’ “ ” – — … • ‹ ›
	clsHalfKana                    // half-width katakana
	clsKana                        // hiragana and katakana
	clsHan                         // kanji
	clsFullPunct                   // ideographic punctuation, full-width forms
	clsOther                       // Greek, Cyrillic, box drawing and other symbols
	clsInvalid                     // not a character of the set
	numCharClasses
)

func classify(r rune) charClass {
	switch {
	case r == utf8.RuneError:
		return clsInvalid
	case 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z':
		return clsLetter
	case '0' <= r && r <= '9':
		return clsDigit
	case r == ' ' || r == '\t' || r == '\r' || r == '\n':
		return clsSpace
	case 0x21 <= r && r <= 0x7E:
		return clsPunct
	case r < 0xA0:
		return clsControl
	case 0x2010 <= r && r <= 0x203A:
		return clsTypo
	case r == 0x20AC || r == 0x2122 || r == 0x0192 || r == 0x02C6 || r == 0x02DC:
		return clsSymbol // € ™ ƒ ˆ ˜
	case r == 0x0152 || r == 0x0153 || r == 0x0160 || r == 0x0161 || r == 0x0178 || r == 0x017D || r == 0x017E:
		return clsRareLetter
	case r <= 0xBF || r == 0xD7 || r == 0xF7:
		return clsSymbol
	case r <= 0xFF:
		return clsAccented
	case 0xFF61 <= r && r <= 0xFF9F:
		return clsHalfKana
	case 0x3040 <= r && r <= 0x30FF:
		return clsKana
	case 0x4E00 <= r && r <= 0x9FFF || 0x3400 <= r && r <= 0x4DBF:
		return clsHan
	case 0x3000 <= r && r <= 0x303F || 0xFF00 <= r && r <= 0xFFEF:
		return clsFullPunct
	}
	return clsOther
}

// charCost is the cost of a character of class c by itself, and
// charPairCost the adjustment when it follows a character of another class.
var (
	charCost     [numCharClasses]float64
	charPairCost [numCharClasses][numCharClasses]float64
)

func init() {
	for c, v := range map[charClass]float64{
		clsLetter: 0, clsDigit: 0, clsSpace: 0, clsPunct: 0,
		clsControl:  12, // C1 codes are terminal controls, never text
		clsAccented: 2,
		// The Windows-1252 letters are rare in Western European text, and
		// their bytes are the most common Shift_JIS lead bytes.
		clsRareLetter: 4,
		clsSymbol:     3.5,
		clsTypo:       3.5, // smart quotes and dashes are rare in typed payloads
		clsHalfKana:   2.5,
		clsKana:       1.5,
		clsHan:        1.5,
		clsFullPunct:  2,
		clsOther:      5,
		clsInvalid:    30,
	} {
		charCost[c] = v
	}
	near := func(a, b charClass, v float64) {
		charPairCost[a][b] += v
		if a != b {
			charPairCost[b][a] += v
		}
	}
	japanese := []charClass{clsKana, clsHan, clsFullPunct}
	// Japanese text borders ASCII words (QRコード, Wi-Fi接続), but a kana or
	// kanji inside one, as ção in Coração would be, is unusual.
	for _, c := range japanese {
		near(c, clsLetter, 0.5)
	}
	near(clsAccented, clsLetter, -1.5)   // é in café, Ö in Öl
	near(clsAccented, clsAccented, 1)    // rare outside ção and the like
	near(clsRareLetter, clsLetter, -0.5) // Œuvre
	near(clsTypo, clsLetter, -1)         // C’est, “Bonjour”
	near(clsTypo, clsSpace, -1)          // Café – Bar
	near(clsSymbol, clsDigit, -1)        // 25°, ½ kg, 9,99 £
	near(clsSymbol, clsSpace, -0.5)      // « Bonjour », ¿Qué?
	near(clsSymbol, clsSymbol, 1)
	// Half-width katakana spells words: starting one costs, continuing it
	// pays back. A lone one between ASCII is as rare as the Latin-1 symbols
	// that share its bytes are common, such as ° in 25°C.
	for c := range charPairCost {
		if charClass(c) != clsHalfKana {
			charPairCost[c][clsHalfKana] += 1.5
		}
	}
	near(clsHalfKana, clsHalfKana, -1.5)
	for _, a := range japanese {
		for _, b := range japanese {
			if a <= b {
				near(a, b, -1) // kana, kanji and their punctuation run together
			}
		}
	}
}

// cp1252 maps bytes 0x80-0x9F of Windows-1252; the five unassigned ones
// stay C1 controls.
var cp1252 = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

// appendWindows1252 decodes Windows-1252.
func appendWindows1252(text, b []byte) []byte {
	for _, c := range b {
		r := rune(c)
		if 0x80 <= c && c <= 0x9F {
			r = cp1252[c-0x80]
		}
		text = utf8.AppendRune(text, r)
	}
	return text
}

// textCost scores how unlikely text is as text: lower is more plausible.
func textCost(text []byte) float64 {
	prev, cost := clsStart, 0.0
	for _, r := range string(text) {
		c := classify(r)
		cost += charCost[c] + charPairCost[prev][c]
		prev = c
	}
	return cost
}

// kanjiPrior is the cost advantage of Shift_JIS in a symbol that also holds
// a Kanji mode segment: only a Japanese encoder writes one.
const kanjiPrior = 8

// guessCharset picks the character set of the undeclared byte segments of a
// symbol, none of which is valid UTF-8. hasKanji reports a Kanji mode
// segment in the symbol.
func guessCharset(segs [][]byte, hasKanji bool) charset {
	var latin, sjis float64
	for _, b := range segs {
		latin += textCost(appendWindows1252(nil, b))
		if !validShiftJIS(b) {
			return charsetLatin1
		}
		sjis += textCost(appendShiftJIS(nil, b))
	}
	if hasKanji {
		sjis -= kanjiPrior
	}
	if sjis < latin {
		return charsetShiftJIS
	}
	return charsetLatin1
}

// validShiftJIS reports whether b is a sequence of Shift_JIS characters
// that the decoder can map: ASCII, half-width katakana and double-byte
// characters of the QR Kanji table.
func validShiftJIS(b []byte) bool {
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c < 0x80, 0xA1 <= c && c <= 0xDF:
		case i+1 < len(b) && validShiftJISTrail(b[i+1]):
			v, ok := shiftJISToKanji(c, b[i+1])
			if !ok {
				return false
			}
			if _, ok := kanjiRune(v); !ok {
				return false
			}
			i++
		default:
			return false
		}
	}
	return true
}

// validShiftJISTrail reports whether c can be the second byte of a
// Shift_JIS double-byte character.
func validShiftJISTrail(c byte) bool {
	return 0x40 <= c && c <= 0x7E || 0x80 <= c && c <= 0xFC
}
