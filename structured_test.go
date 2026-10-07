package qr

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
)

// decodeAll decodes every code and returns the results in order.
func decodeAll(t *testing.T, codes []*Code) []*DecodeResult {
	t.Helper()
	out := make([]*DecodeResult, len(codes))
	for i, c := range codes {
		res, err := Decode(mustImage(t, c, WithScale(3)))
		if err != nil {
			t.Fatalf("symbol %d: %v", i, err)
		}
		out[i] = res
	}
	return out
}

func TestEncodeStructuredRoundTrip(t *testing.T) {
	text := strings.Repeat("Structured append splits long text 0123456789 漢字 é. ", 12)
	codes, err := EncodeStructured(text, WithECC(ECCMedium), WithVersionRange(1, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) < 2 {
		t.Fatalf("got %d symbols, want a sequence", len(codes))
	}

	var parity byte
	for i := 0; i < len(text); i++ {
		parity ^= text[i]
	}
	results := decodeAll(t, codes)
	for i, r := range results {
		if codes[i].Version() > 10 {
			t.Errorf("symbol %d is version %d, above the requested maximum", i, codes[i].Version())
		}
		want := StructuredAppend{Index: i, Total: len(codes), Parity: parity}
		if r.StructuredAppend == nil || *r.StructuredAppend != want {
			t.Fatalf("symbol %d header = %+v, want %+v", i, r.StructuredAppend, want)
		}
		assertEqual(t, ModeStructuredAppend, r.Segments[0].Mode)
	}

	// Join in a shuffled order.
	shuffled := append([]*DecodeResult(nil), results...)
	rand.New(rand.NewSource(1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	got, err := JoinStructuredAppend(shuffled...)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, text, got)
}

func TestEncodeStructuredFitsInOne(t *testing.T) {
	codes, err := EncodeStructured("short")
	if err != nil {
		t.Fatal(err)
	}
	assertLen(t, codes, 1)
	res := decodeAll(t, codes)[0]
	assertNil(t, res.StructuredAppend)
	assertEqual(t, "short", res.Text)
}

func TestEncodeStructuredLimits(t *testing.T) {
	// 16 version-1-L symbols hold at most 16 * 15 bytes after the header.
	if _, err := EncodeStructured(strings.Repeat("x", 16*16), WithECC(ECCLow), WithVersionRange(1, 1)); !errors.Is(err, ErrDataTooLong) {
		t.Errorf("error = %v, want ErrDataTooLong", err)
	}
	if _, err := EncodeStructured("x", WithVersionRange(2, 1)); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
	codes, err := EncodeStructured(strings.Repeat("x", 15*16), WithECC(ECCLow), WithVersionRange(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	assertLen(t, codes, 16)
}

func TestEncodeStructuredSplitsBetweenCharacters(t *testing.T) {
	text := strings.Repeat("é", 60)
	codes, err := EncodeStructured(text, WithECC(ECCLow), WithVersionRange(1, 2), WithSimpleSegmentation())
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range decodeAll(t, codes) {
		if strings.Trim(r.Text, "é") != "" {
			t.Fatalf("symbol %d text %q is not whole characters", i, r.Text)
		}
	}
}

func TestJoinStructuredAppendErrors(t *testing.T) {
	part := func(i, n int, parity byte, text string) *DecodeResult {
		return &DecodeResult{Text: text, StructuredAppend: &StructuredAppend{Index: i, Total: n, Parity: parity}}
	}
	tests := map[string][]*DecodeResult{
		"empty":            nil,
		"not a sequence":   {{Text: "x"}},
		"missing symbol":   {part(0, 3, 7, "a"), part(2, 3, 7, "c")},
		"duplicate symbol": {part(0, 2, 7, "a"), part(0, 2, 7, "a")},
		"other parity":     {part(0, 2, 7, "a"), part(1, 2, 8, "b")},
		"other total":      {part(0, 2, 7, "a"), part(1, 3, 7, "b")},
		"mixed in plain":   {part(0, 2, 7, "a"), {Text: "b"}},
		"index past total": {part(0, 2, 7, "a"), {Text: "b", StructuredAppend: &StructuredAppend{Index: 5, Total: 2, Parity: 7}}},
	}
	for name, parts := range tests {
		if _, err := JoinStructuredAppend(parts...); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: error = %v, want ErrInvalidArgument", name, err)
		}
	}
	got, err := JoinStructuredAppend(part(1, 2, 7, "b"), part(0, 2, 7, "a"))
	assertNoError(t, err)
	assertEqual(t, "ab", got)
}

func TestStructuredAppendHeaderBits(t *testing.T) {
	s := structuredAppendSegment(2, 5, 0xA7)
	// 0010 (index 2) 0100 (total-1 = 4) 10100111 (parity)
	assertEqual(t, []bool{
		false, false, true, false,
		false, true, false, false,
		true, false, true, false, false, true, true, true,
	}, bitsOf(&s.data))
	assertEqual(t, 4+16, totalBits([]Segment{s}, 1))
}

const gs1Text = "0109501101530003" + "17250101" + "10ABC%12" + "\x1d" + "2112345"

func TestGS1RoundTrip(t *testing.T) {
	for _, opts := range [][]EncodeOption{
		{WithGS1()},
		{WithGS1(), WithSimpleSegmentation()},
		{WithGS1(), WithECC(ECCHigh)},
	} {
		code := mustEncode(t, gs1Text, opts...)
		res, err := Decode(mustImage(t, code, WithScale(4)))
		if err != nil {
			t.Fatal(err)
		}
		assertTrue(t, res.GS1)
		assertEqual(t, ModeFNC1, res.Segments[0].Mode)
		assertEqual(t, gs1Text, res.Text)
	}

	// Without WithGS1 the same text is plain data.
	res, err := Decode(mustImage(t, mustEncode(t, gs1Text), WithScale(4)))
	assertNoError(t, err)
	assertFalse(t, res.GS1)
	assertEqual(t, gs1Text, res.Text)
}

func TestGS1Alphanumeric(t *testing.T) {
	assertEqual(t, "AB%%C%D", gs1Alphanumeric("AB%C\x1dD"))
	assertEqual(t, "AB%C\x1dD", string(appendGS1Alphanumeric(nil, []byte("AB%%C%D"))))

	// The separator stays in an alphanumeric segment instead of forcing
	// byte mode.
	segs, err := optSegs("ABC\x1dDEF", ECCLow, 1, 40)
	assertNoError(t, err)
	assertEqual(t, 1, len(segs))
	assertEqual(t, ModeByte, segs[0].Mode())
	c, _ := newEncodeConfig([]EncodeOption{WithGS1()})
	segs, err = optimalSegments("ABC\x1dDEF", c, 0)
	assertNoError(t, err)
	assertEqual(t, 1, len(segs))
	assertEqual(t, ModeAlphanumeric, segs[0].Mode())
}

func TestGS1WithStructuredAppend(t *testing.T) {
	text := strings.Repeat(gs1Text, 6)
	codes, err := EncodeStructured(text, WithGS1(), WithECC(ECCLow), WithVersionRange(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	results := decodeAll(t, codes)
	for i, r := range results {
		assertTrue(t, r.GS1, "symbol %d", i)
	}
	got, err := JoinStructuredAppend(results...)
	assertNoError(t, err)
	assertEqual(t, text, got)
}
