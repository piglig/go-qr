package qr

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestOptimalSegments(t *testing.T) {
	tests := []struct {
		name                   string
		text                   string
		ecl                    ECC
		minVersion, maxVersion int
		wantErr                bool
		wantSegments           []Segment
	}{
		{
			name:       "test with byte text",
			text:       "Hello, World!",
			ecl:        ECCLow,
			minVersion: 1,
			maxVersion: 1,
			wantErr:    false,
			wantSegments: []Segment{
				{
					mode:     ModeByte,
					numChars: 13,
					data: bufFromBits(
						false, true, false, false, true, false, false, false, false, true, true, false, false, true, false,
						true, false, true, true, false, true, true, false, false, false, true, true, false, true, true, false,
						false, false, true, true, false, true, true, true, true, false, false, true, false, true, true, false,
						false, false, false, true, false, false, false, false, false, false, true, false, true, false, true,
						true, true, false, true, true, false, true, true, true, true, false, true, true, true, false, false,
						true, false, false, true, true, false, true, true, false, false, false, true, true, false, false, true,
						false, false, false, false, true, false, false, false, false, true,
					),
				},
			},
		},
		{
			name:       "test with numeric text",
			text:       "314159265358979323846264338327950288419716939937510",
			ecl:        ECCMedium,
			minVersion: 2,
			maxVersion: 2,
			wantErr:    false,
			wantSegments: []Segment{
				{
					mode:     ModeNumeric,
					numChars: 51,
					data: bufFromBits(
						false, true, false, false, true, true, true, false, true, false, false, false, true, false, false,
						true, true, true, true, true, false, true, false, false, false, false, true, false, false, true,
						false, true, false, true, true, false, false, true, true, false, true, true, true, true, false,
						true, false, false, true, true, false, true, false, true, false, false, false, false, true, true,
						true, true, false, true, false, false, true, true, true, false, false, true, false, false, false,
						false, true, false, false, false, false, true, false, true, false, true, false, false, true, false,
						false, true, false, true, false, false, false, true, true, true, true, true, true, false, true, true,
						false, true, true, false, false, true, false, false, true, false, false, false, false, false, false,
						true, true, false, true, false, false, false, true, true, true, false, true, true, false, false, true,
						true, false, false, true, true, true, false, true, false, true, false, true, true, true, true, true,
						false, true, false, true, false, false, true, false, true, true, true, true, true, true, true, true, false,
					),
				},
			},
		},
		{
			name:       "test with alphanumeric mode",
			text:       "DOLLAR-AMOUNT:$39.87 PERCENTAGE:100.00% OPERATIONS:+-*/",
			ecl:        ECCHigh,
			minVersion: 5,
			maxVersion: 5,
			wantErr:    false,
			wantSegments: []Segment{
				{
					mode:     ModeAlphanumeric,
					numChars: 55,
					data: bufFromBits(
						false, true, false, false, true, true, false, false, false, false, true, false, true, true, true,
						true, false, false, false, true, true, false, false, false, true, true, true, false, true, true,
						true, false, true, true, true, true, false, false, true, true, true, true, true, true, false, true,
						true, true, true, true, true, false, true, true, false, true, false, true, false, true, false, true,
						true, true, false, true, true, false, true, false, true, false, false, false, true, false, true,
						true, true, false, true, false, false, false, false, true, false, false, false, false, true, true,
						false, true, true, true, true, true, true, false, false, true, false, true, true, false, true, true,
						true, true, true, true, false, false, true, true, false, true, true, false, true, false, true, false,
						true, false, false, true, false, false, false, true, false, true, false, false, false, true, false,
						true, false, true, false, true, false, false, false, false, true, false, true, false, false, false,
						false, false, true, true, true, false, true, false, false, true, false, false, true, false, true,
						false, true, false, false, false, true, false, false, false, false, false, false, true, false, true,
						true, false, true, false, false, false, false, false, true, false, true, false, true, false, false,
						false, false, false, false, false, false, false, false, false, false, true, true, false, true, true,
						false, true, false, false, true, false, true, false, false, false, true, false, true, false, false,
						false, true, false, true, false, true, false, false, true, false, false, false, true, false, false,
						true, true, true, false, true, true, true, true, true, false, true, true, false, true, false, false,
						false, false, true, false, true, false, false, false, false, true, false, false, true, true, true,
						true, true, true, true, true, true, false, false, true, false, false, true, true, true, false, true,
						false, true, true, true, false, false, true, false, true, false, true, true,
					),
				},
			},
		},
		{
			name:       "test with Kanji mode",
			text:       "「魔法少女まどか☆マギカ」って、　ИАИ　ｄｅｓｕ　κα？",
			ecl:        ECCLow,
			minVersion: 5,
			maxVersion: 5,
			wantErr:    false,
			wantSegments: []Segment{
				{
					mode:     ModeKanji,
					numChars: 29,
					data: bufFromBits(
						false, false, false, false, false, false, false, true, true, false, true, false, true, true, false,
						false, false, false, false, false, false, false, false, false, true, false, false, true, true, true,
						true, true, true, false, false, false, false, false, false, false, true, false, true, false, true,
						true, true, false, true, true, false, true, false, true, false, true, false, true, true, false, true,
						false, true, true, true, false, false, false, false, true, false, true, false, true, true, true,
						false, false, false, false, false, false, true, false, true, false, false, false, true, true, true,
						false, false, false, false, true, false, false, true, false, true, false, false, true, false, false,
						false, false, false, false, true, false, true, true, false, false, true, false, false, false, false,
						true, true, false, true, true, true, true, false, true, false, false, false, false, true, true, false,
						false, false, true, true, false, true, false, false, false, false, true, true, false, false, false,
						true, false, true, false, false, false, false, false, false, false, false, true, true, false, true,
						true, false, false, false, false, false, true, false, true, false, false, false, false, false, true,
						false, false, false, false, true, false, true, false, false, false, true, false, false, false, false,
						false, false, false, false, false, false, false, false, false, false, true, false, false, false, false,
						false, false, false, false, false, false, false, false, false, false, false, false, true, false, false,
						true, false, false, true, false, false, true, false, false, false, true, false, false, true, false, false,
						false, false, false, false, false, false, false, true, false, false, true, false, false, true, false,
						false, true, false, false, false, false, false, false, false, false, false, false, false, false, false,
						false, false, false, false, true, false, false, false, false, false, true, false, false, false, false,
						false, false, true, false, false, false, false, false, true, false, true, false, false, false, false,
						true, false, false, false, true, false, false, true, true, false, false, false, false, true, false,
						false, false, true, false, true, false, true, false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false, true, false, false, false, false, false, true,
						false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true,
						false, false, false, false, false, false, false, false, false, true, false, false, false,
					),
				},
			},
		},
		{
			name:         "test with DataTooLongException",
			text:         "314159265358979323846264338327950288419716939937510",
			ecl:          ECCMedium,
			minVersion:   1,
			maxVersion:   1,
			wantErr:      true,
			wantSegments: nil,
		},
		{
			name:         "test with invalid version",
			text:         "314159265358979323846264338327950288419716939937510",
			ecl:          ECCMedium,
			minVersion:   2,
			maxVersion:   1,
			wantErr:      true,
			wantSegments: nil,
		},
		{
			name:       "test unicode code point near upper limit",
			text:       "\U0010FFFE",
			ecl:        ECCMedium,
			wantErr:    false,
			minVersion: 5,
			maxVersion: 5,
			wantSegments: []Segment{
				{
					mode:     ModeByte,
					numChars: 4,
					data: bufFromBits(true, true, true, true, false, true, false, false, true, false, false, false, true,
						true, true, true, true, false, true, true, true, true, true, true, true, false, true, true, true,
						true, true, false),
				},
			},
		},
		{
			name:         "test with empty text",
			text:         "",
			ecl:          ECCLow,
			minVersion:   1,
			maxVersion:   1,
			wantErr:      false,
			wantSegments: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optSegs(tt.text, tt.ecl, tt.minVersion, tt.maxVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("optimalSegments() error = %v, wantErr %v", err, tt.wantErr)
			}

			assertSegments(t, tt.wantSegments, got)
		})
	}
}

func TestOptimalSegmentsVersionSearch(t *testing.T) {
	tests := []struct {
		name                   string
		text                   string
		ecl                    ECC
		minVersion, maxVersion int
		wantVersion            int // Zero means the payload must not fit.
	}{
		{"fits after version 27", strings.Repeat("a", 740), ECCHigh, 1, 40, 30},
		{"quartile after version 27", strings.Repeat("a", 984), ECCQuartile, 1, 40, 31},
		{"minimum above version 27", strings.Repeat("a", 740), ECCHigh, 28, 40, 30},
		{"fits at version 40", strings.Repeat("a", 1273), ECCHigh, 1, 40, 40},
		{"exceeds version 40", strings.Repeat("a", 1274), ECCHigh, 1, 40, 0},
		{"maximum before version 10", strings.Repeat("a", 40), ECCLow, 1, 2, 0},
		{"fits between checkpoints", strings.Repeat("a", 40), ECCLow, 1, 3, 3},
		{"mixed modes in first range", strings.Repeat("a111111", 5), ECCLow, 1, 40, 2},
		{"mixed modes with tight maximum", strings.Repeat("a111111", 5), ECCLow, 1, 2, 2},
		{"fixed version fits", strings.Repeat("a", 740), ECCHigh, 30, 30, 30},
		{"fixed version too small", strings.Repeat("a", 740), ECCHigh, 28, 28, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs, err := optSegs(tt.text, tt.ecl, tt.minVersion, tt.maxVersion)
			if tt.wantVersion == 0 {
				if !errors.Is(err, ErrDataTooLong) {
					t.Fatalf("optimalSegments() error = %v, want ErrDataTooLong", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			qr, err := EncodeSegments(segs, WithECC(tt.ecl), WithVersionRange(tt.minVersion, tt.maxVersion), WithoutECCBoost())
			if err != nil {
				t.Fatal(err)
			}
			if got := qr.Version(); got != tt.wantVersion {
				t.Fatalf("encoded version = %d, want %d", got, tt.wantVersion)
			}
		})
	}
}

func TestOptimalSegmentsCapacityAtEveryVersion(t *testing.T) {
	for _, ecl := range []ECC{ECCLow, ECCMedium, ECCQuartile, ECCHigh} {
		for version := MinVersion; version <= MaxVersion; version++ {
			t.Run(fmt.Sprintf("ecc%d/version%d", ecl, version), func(t *testing.T) {
				// Lowercase ASCII requires byte mode. Choose the largest byte
				// payload fitting this version, accounting for its segment header.
				capacity := numDataCodewords(version, ecl) * 8
				n := (capacity - 4 - ModeByte.charCountBits(version)) / 8
				text := strings.Repeat("a", n)
				segs, err := optSegs(text, ecl, MinVersion, version)
				if err != nil {
					t.Fatalf("maximum fitting payload: %v", err)
				}
				bits := totalBits(segs, version)
				if bits < 0 || bits > capacity {
					t.Fatalf("returned segments use %d bits, capacity is %d", bits, capacity)
				}
				if _, err := optSegs(text+"a", ecl, MinVersion, version); !errors.Is(err, ErrDataTooLong) {
					t.Fatalf("one byte over capacity: error = %v, want ErrDataTooLong", err)
				}
			})
		}
	}
}

// optSegs runs the optimal segmenter with the given ECC level and version range.
func optSegs(text string, ecc ECC, minVer, maxVer int) ([]Segment, error) {
	c, err := newEncodeConfig([]EncodeOption{WithECC(ecc), WithVersionRange(minVer, maxVer)})
	if err != nil {
		return nil, err
	}
	return optimalSegments(text, c, 0)
}

func assertSegments(t *testing.T, want, got []Segment) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("got %d segments, want %d", len(got), len(want))
	}
	for i := range want {
		assertSegment(t, want[i], got[i])
	}
}

func TestOptimalSegmentsMixedModes(t *testing.T) {
	segs, err := optSegs("1234567890ABCDEFGHIJ漢字漢字漢字hello", ECCLow, 1, 40)
	assertNoError(t, err)
	var modes []Mode
	for _, s := range segs {
		modes = append(modes, s.Mode())
	}
	assertEqual(t, []Mode{ModeNumeric, ModeAlphanumeric, ModeKanji, ModeByte}, modes)
}

func TestOptimalSegmentsInvalidUTF8(t *testing.T) {
	text := "ok\xff\xfe"
	segs, err := optSegs(text, ECCLow, 1, 40)
	assertNoError(t, err)
	if len(segs) != 1 || segs[0].Mode() != ModeByte || segs[0].NumChars() != len(text) {
		t.Fatalf("invalid UTF-8 should be one byte segment of %d bytes, got %+v", len(text), segs)
	}
}

func TestOptimalSegmentsTooManyChars(t *testing.T) {
	_, err := optSegs(strings.Repeat("1", maxOptimalChars+1), ECCLow, 1, 40)
	if !errors.Is(err, ErrDataTooLong) {
		t.Fatalf("error = %v, want ErrDataTooLong", err)
	}
}

func BenchmarkOptimalSegments(b *testing.B) {
	text := strings.Repeat("Order 12345678 漢字 HTTPS://EXAMPLE.COM/x?y=1 ", 20)
	c, _ := newEncodeConfig(nil)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := optimalSegments(text, c, 0); err != nil {
			b.Fatal(err)
		}
	}
}
