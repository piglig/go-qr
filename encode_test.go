package qr

import (
	"errors"
	"strings"
	"testing"
)

func TestEncodeOptionValidation(t *testing.T) {
	tests := []struct {
		name string
		opts []EncodeOption
		want error
	}{
		{"unknown ECC", []EncodeOption{WithECC(ECC(4))}, ErrInvalidArgument},
		{"negative ECC", []EncodeOption{WithECC(ECC(-1))}, ErrInvalidArgument},
		{"min above max", []EncodeOption{WithVersionRange(5, 4)}, ErrInvalidVersion},
		{"min below 1", []EncodeOption{WithVersionRange(0, 4)}, ErrInvalidVersion},
		{"max above 40", []EncodeOption{WithVersionRange(1, 41)}, ErrInvalidVersion},
		{"mask too large", []EncodeOption{WithMask(8)}, ErrInvalidArgument},
		{"mask negative", []EncodeOption{WithMask(-1)}, ErrInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Encode("x", tt.opts...); !errors.Is(err, tt.want) {
				t.Errorf("Encode error = %v, want %v", err, tt.want)
			}
			if _, err := EncodeBytes([]byte("x"), tt.opts...); !errors.Is(err, tt.want) {
				t.Errorf("EncodeBytes error = %v, want %v", err, tt.want)
			}
			if _, err := EncodeSegments(nil, tt.opts...); !errors.Is(err, tt.want) {
				t.Errorf("EncodeSegments error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEncodeDefaults(t *testing.T) {
	code, err := Encode("HELLO WORLD 1234567890 HELLO WORLD")
	assertNoError(t, err)
	assertEqual(t, 2, code.Version())
	if code.ECC() < ECCMedium {
		t.Errorf("default ECC = %v, want at least M", code.ECC())
	}
	if code.Mask() < 0 || code.Mask() > 7 {
		t.Errorf("Mask() = %d, want 0..7", code.Mask())
	}
	assertEqual(t, 4*code.Version()+17, code.Size())
}

func TestEncodeECCBoost(t *testing.T) {
	boosted, err := Encode("1", WithECC(ECCLow))
	assertNoError(t, err)
	assertEqual(t, ECCHigh, boosted.ECC())

	exact, err := Encode("1", WithECC(ECCLow), WithoutECCBoost())
	assertNoError(t, err)
	assertEqual(t, ECCLow, exact.ECC())
}

func TestEncodeFixedVersionAndMask(t *testing.T) {
	code, err := Encode("hi", WithVersionRange(7, 7), WithMask(5))
	assertNoError(t, err)
	assertEqual(t, 7, code.Version())
	assertEqual(t, 5, code.Mask())
}

func TestEncodeMatchesExplicitSegments(t *testing.T) {
	text := "ABC123漢字abc"
	got, err := Encode(text, WithECC(ECCLow))
	assertNoError(t, err)
	segs, err := optSegs(text, ECCLow, MinVersion, MaxVersion)
	assertNoError(t, err)
	want, err := EncodeSegments(segs, WithECC(ECCLow))
	assertNoError(t, err)
	assertEqual(t, want, got)
}

func TestEncodeUTF8ECI(t *testing.T) {
	// ASCII text gets no ECI segment, so the option changes nothing.
	plain, _ := Encode("hello", WithMask(0))
	withECI, _ := Encode("hello", WithMask(0), WithUTF8ECI())
	assertEqual(t, plain, withECI)

	// Non-ASCII text gets an ECI 26 prefix: 12 more bits than without it.
	eci, _ := ECISegment(eciUTF8)
	text := "héllo wörld"
	code, err := Encode(text, WithUTF8ECI(), WithECC(ECCLow), WithoutECCBoost(), WithMask(0))
	assertNoError(t, err)
	segs, _ := optSegs(text, ECCLow, MinVersion, MaxVersion)
	want, err := EncodeSegments(append([]Segment{eci}, segs...), WithECC(ECCLow), WithoutECCBoost(), WithMask(0))
	assertNoError(t, err)
	assertEqual(t, want, code)
}

func TestEncodeUTF8ECICountsTowardCapacity(t *testing.T) {
	// Version 1-L holds 152 data bits: a 17-byte segment (12 + 136 bits)
	// fits, but not with the 12-bit ECI designator in front of it.
	text := strings.Repeat("é", 8) + "a"
	code, err := Encode(text, WithECC(ECCLow), WithVersionRange(1, 1))
	assertNoError(t, err)
	assertEqual(t, 1, code.Version())
	if _, err := Encode(text, WithECC(ECCLow), WithVersionRange(1, 1), WithUTF8ECI()); !errors.Is(err, ErrDataTooLong) {
		t.Errorf("error = %v, want ErrDataTooLong", err)
	}
}

func TestEncodeBytesUTF8ECI(t *testing.T) {
	code, err := EncodeBytes([]byte{0xC3, 0xA9}, WithUTF8ECI(), WithECC(ECCLow), WithMask(2))
	assertNoError(t, err)
	eci, _ := ECISegment(eciUTF8)
	want, _ := EncodeSegments([]Segment{eci, BytesSegment([]byte{0xC3, 0xA9})}, WithECC(ECCLow), WithMask(2))
	assertEqual(t, want, code)
}

func TestEncodeSegmentsTooLongCount(t *testing.T) {
	// A byte segment longer than any 16-bit count indicator can describe.
	_, err := EncodeSegments([]Segment{BytesSegment(make([]byte, 1<<16))})
	if !errors.Is(err, ErrDataTooLong) {
		t.Fatalf("error = %v, want ErrDataTooLong", err)
	}
}

func TestECCString(t *testing.T) {
	for e, want := range map[ECC]string{ECCLow: "L", ECCMedium: "M", ECCQuartile: "Q", ECCHigh: "H", ECC(9): "ECC(9)"} {
		assertEqual(t, want, e.String())
	}
}

func TestCodeModuleOutOfRange(t *testing.T) {
	code, err := Encode("x")
	assertNoError(t, err)
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {code.Size(), 0}, {0, code.Size()}} {
		if code.Module(p[0], p[1]) {
			t.Errorf("Module(%d, %d) = true outside the symbol", p[0], p[1])
		}
	}
	if !code.Module(0, 0) {
		t.Error("Module(0, 0) should be the dark finder corner")
	}
}

func BenchmarkEncode(b *testing.B) {
	for _, c := range []struct{ name, text string }{
		{"v1", "HELLO WORLD"},
		{"v4", "https://github.com/piglig/go-qr?ref=bench&v=1"},
		{"v20", strings.Repeat("The quick brown fox jumps over the lazy dog. ", 15)},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Encode(c.text); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
