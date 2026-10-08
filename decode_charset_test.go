package qr

import (
	"testing"
	"unicode/utf8"
)

// toShiftJIS encodes ASCII, half-width katakana and characters of the QR
// Kanji table in Shift_JIS.
func toShiftJIS(t testing.TB, s string) []byte {
	t.Helper()
	var b []byte
	for _, r := range s {
		switch {
		case r < 0x80:
			b = append(b, byte(r))
		case 0xFF61 <= r && r <= 0xFF9F:
			b = append(b, byte(r-0xFF61+0xA1))
		default:
			v, ok := kanjiValue(r)
			if !ok {
				t.Fatalf("%q has no Shift_JIS encoding", r)
			}
			b1, b2 := kanjiToShiftJIS(v)
			b = append(b, b1, b2)
		}
	}
	return b
}

// toWindows1252 encodes s in Windows-1252.
func toWindows1252(t testing.TB, s string) []byte {
	t.Helper()
	var b []byte
next:
	for _, r := range s {
		if r < 0x80 || 0xA0 <= r && r <= 0xFF {
			b = append(b, byte(r))
			continue
		}
		for i, c := range cp1252 {
			if c == r {
				b = append(b, byte(0x80+i))
				continue next
			}
		}
		t.Fatalf("%q is not in Windows-1252", r)
	}
	return b
}

// charsetCorpus is text as encoders write it without an ECI: Japanese in
// Shift_JIS and Western European text in ISO-8859-1, from words and short
// strings, where guessing is hardest, to typical payloads.
var charsetCorpus = struct{ japanese, latin []string }{
	japanese: []string{
		// kanji and kana
		"東京都千代田区丸の内1-1-1", "お問い合わせはこちら", "株式会社サンプル", "ありがとうございました",
		"新宿駅", "日本", "東", "山田", "愛", "テスト", "モバイル版はこちら http://m.example.jp",
		"【期間限定】キャンペーン", "〒100-0001 東京都", "＊ご注意＊", "第2回", "Wi-Fi接続",
		"ポイント2倍", "営業時間 10:00〜19:00", "品番:A-123 色:赤", "TEL:03-1234-5678 担当:佐藤",
		"MECARD:N:山田,太郎;TEL:0312345678;;", "WIFI:S:ゲスト;T:WPA;P:password;;",
		"BEGIN:VCARD\r\nN:鈴木;一郎\r\nEND:VCARD", "Google モバイル\r\nhttp://google.jp",
		"[外側QRコード]", "食べ放題", "春", "円", "ご予約", "詳しくはWebで",
		// half-width katakana
		"ﾃﾞｻﾞｲﾝQR", "ｶﾞｲﾄﾞﾌﾞｯｸ", "ﾎﾟｲﾝﾄ", "ｸｰﾎﾟﾝ", "ﾃｽﾄ", "ｺｰﾄﾞ", "ｶﾌｪ", "ﾗｰﾒﾝ",
		"*ﾃﾞｻﾞｲﾝQR*\r\nhttp://d-qr.net/ex/", "ｲﾗｽﾄ入りｶﾗｰQRｺｰﾄﾞ", "ｱ", "ﾒｰﾙ", "ｾｰﾙ中",
		// short half-width words
		"ﾒﾓ", "ｷｰ", "ｾﾙ", "ﾄﾞｱ", "ID:ｷｰ", "ﾛｸﾞｲﾝ", "ﾍﾙﾌﾟ",
		// kanji with lead bytes above 0x9F
		"蕎麦", "鰻重", "嬉しい", "詳細", "鍵",
	},
	latin: []string{
		// words
		"café", "Ångström", "naïve résumé", "Größe", "señor Ñandú", "élève", "Crème brûlée",
		"Müller", "José García", "São Paulo", "Ærø", "Øresund", "Málaga", "Zürich", "Straße 12",
		"Düsseldorf", "Fußball", "Jalapeño", "Pokémon", "déjà vu", "Ça va?", "Reykjavík", "Þór",
		"Ísland", "Ålesund", "Mañana", "Coração", "Fête", "Hôtel Élysée", "Bäckerei Schäfer",
		"Öl", "Ex", "Ñ", "é", "Ø",
		// capitals
		"ÉCOLE", "ÉTÉ", "NOËL", "CAFÉ", "SÃO TOMÉ", "MÜNCHEN", "ÅRHUS",
		// symbols
		"°C", "25°", "© 2024 Société Générale", "Preis: 9,99 £", "±0,5 mm", "10 µg", "½ kg",
		"« Bonjour »", "¿Qué?", "¡Hola!", "Größe: L × 40 cm",
		// payloads
		"MECARD:N:Müller,Jürgen;;", "WIFI:S:Café Ü;T:WPA;P:pässwörd;;",
		"BEGIN:VCARD\r\nN:Pérez;Ana\r\nORG:Señorita Ltda.\r\nEND:VCARD", "Grüße aus Köln",
		// Windows-1252
		"“Bonjour”", "Prix : 12,50 €", "Café – Bar", "Œuvre", "C’est la vie", "Wait…", "Brand™",
		"‘Quoted’ text", "Total: 12,50 € HT", "Páginas 1–3",
	},
}

// charsetAccuracy reports how many corpus strings guess reads in their own
// character set, and which it misses.
func charsetAccuracy(t *testing.T, name string, guess func(b []byte) charset) (int, []string) {
	jp, lat := 0, 0
	var misses []string
	for _, s := range charsetCorpus.japanese {
		if guess(toShiftJIS(t, s)) == charsetShiftJIS {
			jp++
		} else {
			misses = append(misses, "ja "+s)
		}
	}
	for _, s := range charsetCorpus.latin {
		b := toWindows1252(t, s)
		if utf8.Valid(b) || guess(b) == charsetLatin1 {
			lat++
		} else {
			misses = append(misses, "la "+s)
		}
	}
	t.Logf("%s: Japanese %d/%d, Western %d/%d; misses %q", name, jp, len(charsetCorpus.japanese), lat, len(charsetCorpus.latin), misses)
	return jp + lat, misses
}

// ambiguousCharset lists corpus strings whose bytes read plausibly in both
// character sets: one or two half-width katakana are also Latin-1 letters
// or symbols, and Œu is also a kanji.
var ambiguousCharset = map[string]bool{"ja ｱ": true, "ja ﾒﾓ": true, "la Œuvre": true}

func legacyGuess(b []byte) charset {
	if looksShiftJIS(b) {
		return charsetShiftJIS
	}
	return charsetLatin1
}

func modelGuess(b []byte) charset { return guessCharset([][]byte{b}, false) }

// TestCharsetModel checks the model on the corpus it was designed with and
// on two written afterwards, and that it reads at least as many strings
// correctly as the rule of v2.4.1 on each.
func TestCharsetModel(t *testing.T) {
	saved := charsetCorpus
	defer func() { charsetCorpus = saved }()
	for _, c := range []struct {
		name   string
		corpus struct{ japanese, latin []string }
	}{{"design", saved}, {"held out", charsetHeldOut}, {"held out 2", charsetHeldOut2}} {
		charsetCorpus = c.corpus
		got, misses := charsetAccuracy(t, "model, "+c.name, modelGuess)
		for _, m := range misses {
			if !ambiguousCharset[m] {
				t.Errorf("%s corpus: %q read in the wrong character set", c.name, m)
			}
		}
		if legacy, _ := charsetAccuracy(t, "v2.4.1 rule, "+c.name, legacyGuess); got < legacy {
			t.Errorf("%s corpus: model reads %d, the v2.4.1 rule %d", c.name, got, legacy)
		}
	}
}

// charsetHeldOut was written after the model's costs were fixed and is not
// used to tune them.
var charsetHeldOut = struct{ japanese, latin []string }{
	japanese: []string{
		"本日のおすすめ", "大阪府大阪市北区梅田3丁目", "会員登録はこちらから", "賞味期限 2026年10月31日",
		"天ぷら定食", "駐車場あり", "禁煙", "電話番号", "抽選で10名様にプレゼント", "取扱説明書",
		"ｱﾌﾟﾘをﾀﾞｳﾝﾛｰﾄﾞ", "ｷｬﾝﾍﾟｰﾝ実施中", "ﾏﾙﾁﾒﾃﾞｨｱ", "ﾊﾟｽﾜｰﾄﾞ", "ｻｲｽﾞ:M", "ｶﾗｰ:ﾌﾞﾙｰ",
		"MECARD:N:田中,花子;EMAIL:hanako@example.jp;;", "LINEで友だち追加", "No.12345 京都",
		"お客様各位", "〒530-0001", "こんにちは", "雪", "猫", "ﾈｺ", "ｲﾇ",
	},
	latin: []string{
		"Bon appétit", "Frühstück inklusive", "Peña Nieto", "Ação de graças", "Città di Firenze",
		"Göteborg", "Højbro Plads", "Ölçek", "Fiancée", "Kaffee & Kuchen: 4,50 €", "Tamaño: 30×40 cm",
		"Qualität seit 1923", "CRÊPERIE", "PÂTISSERIE", "BIÈRE", "ESPAÑA", "QUÉBEC", "Wärme",
		"„Guten Tag“", "– 20 % –", "naïveté", "Façade", "Garçon", "señal", "Øl", "Þú",
		"Préférences…", "Size: 10½", "Temp: -5 °C", "€ 19,99", "© Ånes AS",
	},
}

// charsetHeldOut2 was written after the last change to the costs.
var charsetHeldOut2 = struct{ japanese, latin []string }{
	japanese: []string{
		"鳥", "海", "本", "空港", "地図", "店舗情報", "クーポン券", "割引", "送料無料", "ご来店お待ちしております",
		"ｿﾌﾄｸﾘｰﾑ", "ﾒﾆｭｰ", "ｳｪﾌﾞ", "ﾁｹｯﾄ", "ﾄｲﾚ", "QRｺｰﾄﾞ決済", "WIFI:S:ｹﾞｽﾄ;T:WPA;P:abc12345;;",
		"MECARD:N:伊藤,健;TEL:090-1234-5678;;", "博多ラーメン", "北海道札幌市",
	},
	latin: []string{
		"Crêpe", "Bière", "Müsli", "Señora", "Ação", "Garçon", "Øst", "Åbo", "Äpfel", "Größe M",
		"ÉDITION", "CAFÉ-RESTAURANT", "« Menu du jour »", "Prix: 8 €", "“Welcome”", "It’s here",
		"10–12 Uhr", "º", "3ª", "Größte Auswahl", "Lyngbyvej 2, 2100 København Ø",
	},
}

// looksShiftJIS is the rule of v2.4.1, kept to compare with: it reports whether a byte segment without an ECI, which is not
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

// TestDecodeCharsetSymbol covers the inference through Decode: a Kanji mode
// segment marks the symbol as Japanese, so a byte segment too short to tell
// is read as Shift_JIS, and Windows-1252 punctuation reads as such.
func TestDecodeCharsetSymbol(t *testing.T) {
	kanji, err := KanjiSegment("予約")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		segs []Segment
		want string
	}{
		{"short katakana alone", []Segment{BytesSegment([]byte("\xd2\xd3"))}, "ÒÓ"},
		{"short katakana with Kanji", []Segment{kanji, BytesSegment([]byte(" \xd2\xd3"))}, "予約 ﾒﾓ"},
		{"Windows-1252 quotes", []Segment{BytesSegment([]byte("\x93Bonjour\x94"))}, "“Bonjour”"},
		{"two segments, one clear", []Segment{BytesSegment([]byte("\xc3\xde\xbb\xde\xb2\xdd")), mustNumericSegment("123"), BytesSegment([]byte("\xd2\xd3"))}, "ﾃﾞｻﾞｲﾝ123ﾒﾓ"},
	} {
		code, err := EncodeSegments(c.segs)
		if err != nil {
			t.Fatal(err)
		}
		res, err := Decode(mustImage(t, code, WithScale(4)))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if res.Text != c.want {
			t.Errorf("%s: got %q, want %q", c.name, res.Text, c.want)
		}
	}
}

// mustNumericSegment is NumericSegment for literals in tests.
func mustNumericSegment(digits string) Segment {
	s, err := NumericSegment(digits)
	if err != nil {
		panic(err)
	}
	return s
}
