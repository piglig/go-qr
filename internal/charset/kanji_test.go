package charset

import "testing"

func TestShiftJISRoundTrip(t *testing.T) {
	for v := 0; v < len(kanjiToUnicode); v++ {
		if _, ok := KanjiRune(v); !ok {
			continue
		}
		b1, b2 := KanjiToShiftJIS(v)
		got, ok := ShiftJISToKanji(b1, b2)
		if !ok || got != v {
			t.Fatalf("value %d -> SJIS %02X%02X -> %d, %v", v, b1, b2, got, ok)
		}
	}
	if _, ok := ShiftJISToKanji(0xA0, 0x40); ok {
		t.Error("0xA040 is not a Kanji-mode code")
	}
}
