package go_qr

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeSegmentsOptimally(t *testing.T) {
	tests := []struct {
		name                   string
		text                   string
		ecl                    Ecc
		minVersion, maxVersion int
		wantErr                bool
		wantSegments           []*QrSegment
	}{
		{
			name:       "test with byte text",
			text:       "Hello, World!",
			ecl:        Low,
			minVersion: 1,
			maxVersion: 1,
			wantErr:    false,
			wantSegments: []*QrSegment{
				{
					mode:     Byte,
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
			ecl:        Medium,
			minVersion: 2,
			maxVersion: 2,
			wantErr:    false,
			wantSegments: []*QrSegment{
				{
					mode:     Numeric,
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
			ecl:        High,
			minVersion: 5,
			maxVersion: 5,
			wantErr:    false,
			wantSegments: []*QrSegment{
				{
					mode:     Alphanumeric,
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
			ecl:        Low,
			minVersion: 5,
			maxVersion: 5,
			wantErr:    false,
			wantSegments: []*QrSegment{
				{
					mode:     Kanji,
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
			ecl:          Medium,
			minVersion:   1,
			maxVersion:   1,
			wantErr:      true,
			wantSegments: nil,
		},
		{
			name:         "test with invalid version",
			text:         "314159265358979323846264338327950288419716939937510",
			ecl:          Medium,
			minVersion:   2,
			maxVersion:   1,
			wantErr:      true,
			wantSegments: nil,
		},
		{
			name:       "test unicode code point near upper limit",
			text:       "\U0010FFFE",
			ecl:        Medium,
			wantErr:    false,
			minVersion: 5,
			maxVersion: 5,
			wantSegments: []*QrSegment{
				{
					mode:     Byte,
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
			ecl:          Low,
			minVersion:   1,
			maxVersion:   1,
			wantErr:      false,
			wantSegments: []*QrSegment{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeSegmentsOptimally(tt.text, tt.ecl, tt.minVersion, tt.maxVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeSegmentsOptimally() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.wantSegments, got)
		})
	}
}

func TestCountUtf8Bytes(t *testing.T) {
	tests := []struct {
		name     string
		cp       int
		wantErr  bool
		wantData int
	}{
		{
			name:     "test with negative value",
			cp:       -1,
			wantErr:  true,
			wantData: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := countUtf8Bytes(tt.cp)
			if (err != nil) != tt.wantErr {
				t.Errorf("countUtf8Bytes() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.wantData, got)
		})
	}
}

func TestMakeSegmentsOptimallyVersionSearch(t *testing.T) {
	tests := []struct {
		name                   string
		text                   string
		ecl                    Ecc
		minVersion, maxVersion int
		wantVersion            int // Zero means the payload must not fit.
	}{
		{"fits after version 27", strings.Repeat("a", 740), High, 1, 40, 30},
		{"quartile after version 27", strings.Repeat("a", 984), Quartile, 1, 40, 31},
		{"minimum above version 27", strings.Repeat("a", 740), High, 28, 40, 30},
		{"fits at version 40", strings.Repeat("a", 1273), High, 1, 40, 40},
		{"exceeds version 40", strings.Repeat("a", 1274), High, 1, 40, 0},
		{"maximum before version 10", strings.Repeat("a", 40), Low, 1, 2, 0},
		{"fits between checkpoints", strings.Repeat("a", 40), Low, 1, 3, 3},
		{"mixed modes in first range", strings.Repeat("a111111", 5), Low, 1, 40, 2},
		{"mixed modes with tight maximum", strings.Repeat("a111111", 5), Low, 1, 2, 2},
		{"fixed version fits", strings.Repeat("a", 740), High, 30, 30, 30},
		{"fixed version too small", strings.Repeat("a", 740), High, 28, 28, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs, err := MakeSegmentsOptimally(tt.text, tt.ecl, tt.minVersion, tt.maxVersion)
			if tt.wantVersion == 0 {
				if !errors.Is(err, ErrDataTooLong) {
					t.Fatalf("MakeSegmentsOptimally() error = %v, want ErrDataTooLong", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			qr, err := EncodeSegments(segs, tt.ecl, tt.minVersion, tt.maxVersion, -1, false)
			if err != nil {
				t.Fatal(err)
			}
			if got := (qr.Size() - 17) / 4; got != tt.wantVersion {
				t.Fatalf("encoded version = %d, want %d", got, tt.wantVersion)
			}
		})
	}
}

func TestMakeSegmentsOptimallyCapacityAtEveryVersion(t *testing.T) {
	for _, ecl := range []Ecc{Low, Medium, Quartile, High} {
		for version := MinVersion; version <= MaxVersion; version++ {
			t.Run(fmt.Sprintf("ecc%d/version%d", ecl, version), func(t *testing.T) {
				// Lowercase ASCII requires byte mode. Choose the largest byte
				// payload fitting this version, accounting for its segment header.
				capacity := getNumDataCodewords(version, ecl) * 8
				n := (capacity - 4 - Byte.numCharCountBits(version)) / 8
				text := strings.Repeat("a", n)
				segs, err := MakeSegmentsOptimally(text, ecl, MinVersion, version)
				if err != nil {
					t.Fatalf("maximum fitting payload: %v", err)
				}
				bits := getTotalBits(segs, version)
				if bits < 0 || bits > capacity {
					t.Fatalf("returned segments use %d bits, capacity is %d", bits, capacity)
				}
				if _, err := MakeSegmentsOptimally(text+"a", ecl, MinVersion, version); !errors.Is(err, ErrDataTooLong) {
					t.Fatalf("one byte over capacity: error = %v, want ErrDataTooLong", err)
				}
			})
		}
	}
}
