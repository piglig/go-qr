package qr

import (
	"testing"
)

func TestAlphanumericSegment(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantErr  bool
		wantData Segment
	}{
		{
			name:    "test with a digit",
			data:    "0",
			wantErr: false,
			wantData: Segment{
				mode:     ModeAlphanumeric,
				numChars: 1,
				data:     bufFromBits(false, false, false, false, false, false),
			},
		},
		{
			name:    "test with normal digits",
			data:    "123456",
			wantErr: false,
			wantData: Segment{
				mode:     ModeAlphanumeric,
				numChars: 6,
				data: bufFromBits(false, false, false, false, false, true, false, true, true, true, true, false, false,
					false, true, false, false, false, true, false, true, true, false, false, false, true, true, true, false,
					false, true, true, true),
			},
		},
		{
			name:    "test with empty data",
			data:    "",
			wantErr: false,
			wantData: Segment{
				mode:     ModeAlphanumeric,
				numChars: 0,
				data:     bufFromBits(),
			},
		},
		{
			name:    "test with a lower case letter",
			data:    "a",
			wantErr: true,
		},
		{
			name:    "test with a uppercase letter",
			data:    "A",
			wantErr: false,
			wantData: Segment{
				mode:     ModeAlphanumeric,
				numChars: 1,
				data:     bufFromBits(false, false, true, false, true, false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AlphanumericSegment(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("segment constructor error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				assertSegment(t, tt.wantData, got)
			}
		})
	}
}

func TestNumericSegment(t *testing.T) {
	tests := []struct {
		name     string
		digits   string
		wantErr  bool
		wantData Segment
	}{
		{
			name:    "test with normal digits",
			digits:  "314159265358979323846264338327950288419716939937510",
			wantErr: false,
			wantData: Segment{
				mode:     ModeNumeric,
				numChars: 51,
				data: bufFromBits(false, true, false, false, true, true, true, false, true, false, false, false, true,
					false, false, true, true, true, true, true, false, true, false, false, false, false, true, false, false,
					true, false, true, false, true, true, false, false, true, true, false, true, true, true, true, false,
					true, false, false, true, true, false, true, false, true, false, false, false, false, true, true, true,
					true, false, true, false, false, true, true, true, false, false, true, false, false, false, false, true,
					false, false, false, false, true, false, true, false, true, false, false, true, false, false, true, false,
					true, false, false, false, true, true, true, true, true, true, false, true, true, false, true, true,
					false, false, true, false, false, true, false, false, false, false, false, false, true, true, false,
					true, false, false, false, true, true, true, false, true, true, false, false, true, true, false, false,
					true, true, true, false, true, false, true, false, true, true, true, true, true, false, true, false, true,
					false, false, true, false, true, true, true, true, true, true, true, true, false),
			},
		},
		{
			name:    "test with empty digit",
			digits:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NumericSegment(tt.digits)
			if (err != nil) != tt.wantErr {
				t.Errorf("segment constructor error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				assertSegment(t, tt.wantData, got)
			}
		})
	}
}

func TestBytesSegment(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantErr  bool
		wantData Segment
	}{
		{
			name:    "test with non-nil data",
			data:    []byte("https://www.github.com/piglig"),
			wantErr: false,
			wantData: Segment{
				mode:     ModeByte,
				numChars: 29,
				data: bufFromBits(false, true, true, false, true, false, false, false, false, true, true, true, false, true,
					false, false, false, true, true, true, false, true, false, false, false, true, true, true, false, false,
					false, false, false, true, true, true, false, false, true, true, false, false, true, true, true, false,
					true, false, false, false, true, false, true, true, true, true, false, false, true, false, true, true,
					true, true, false, true, true, true, false, true, true, true, false, true, true, true, false, true, true,
					true, false, true, true, true, false, true, true, true, false, false, true, false, true, true, true,
					false, false, true, true, false, false, true, true, true, false, true, true, false, true, false, false,
					true, false, true, true, true, false, true, false, false, false, true, true, false, true, false, false,
					false, false, true, true, true, false, true, false, true, false, true, true, false, false, false, true,
					false, false, false, true, false, true, true, true, false, false, true, true, false, false, false, true,
					true, false, true, true, false, true, true, true, true, false, true, true, false, true, true, false, true,
					false, false, true, false, true, true, true, true, false, true, true, true, false, false, false, false,
					false, true, true, false, true, false, false, true, false, true, true, false, false, true, true, true,
					false, true, true, false, true, true, false, false, false, true, true, false, true, false, false, true,
					false, true, true, false, false, true, true, true),
			},
		},
		{
			name:     "test with nil data",
			data:     nil,
			wantData: Segment{mode: ModeByte},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSegment(t, tt.wantData, BytesSegment(tt.data))
		})
	}
}

func TestKanjiSegment(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantErr  bool
		wantData Segment
	}{
		{
			name:    "test with Kanji data",
			data:    "「魔法少女まどか☆マギカ」って、　ИАИ　ｄｅｓｕ　κα？",
			wantErr: false,
			wantData: Segment{
				mode:     ModeKanji,
				numChars: 29,
				data: bufFromBits(false, false, false, false, false, false, false, true, true, false, true, false, true, true,
					false, false, false, false, false, false, false, false, false, false, true, false, false, true, true,
					true, true, true, true, false, false, false, false, false, false, false, true, false, true, false, true,
					true, true, false, true, true, false, true, false, true, false, true, false, true, true, false, true,
					false, true, true, true, false, false, false, false, true, false, true, false, true, true, true, false,
					false, false, false, false, false, true, false, true, false, false, false, true, true, true, false, false,
					false, false, true, false, false, true, false, true, false, false, true, false, false, false, false,
					false, false, true, false, true, true, false, false, true, false, false, false, false, true, true, false,
					true, true, true, true, false, true, false, false, false, false, true, true, false, false, false, true,
					true, false, true, false, false, false, false, true, true, false, false, false, true, false, true, false,
					false, false, false, false, false, false, false, true, true, false, true, true, false, false, false,
					false, false, true, false, true, false, false, false, false, false, true, false, false, false, false,
					true, false, true, false, false, false, true, false, false, false, false, false, false, false, false,
					false, false, false, false, false, false, true, false, false, false, false, false, false, false, false,
					false, false, false, false, false, false, false, false, true, false, false, true, false, false, true,
					false, false, true, false, false, false, true, false, false, true, false, false, false, false, false,
					false, false, false, false, true, false, false, true, false, false, true, false, false, true, false,
					false, false, false, false, false, false, false, false, false, false, false, false, false, false, false,
					false, true, false, false, false, false, false, true, false, false, false, false, false, false, true,
					false, false, false, false, false, true, false, true, false, false, false, false, true, false, false,
					false, true, false, false, true, true, false, false, false, false, true, false, false, false, true,
					false, true, false, true, false, false, false, false, false, false, false, false, false, false, false,
					false, false, false, false, false, true, false, false, false, false, false, true, false, false, false,
					false, false, false, false, true, true, true, true, true, true, true, true, true, false, false, false,
					false, false, false, false, false, false, true, false, false, false),
			},
		},
		{
			name:    "test with nil data",
			data:    "",
			wantErr: false,
			wantData: Segment{
				mode:     ModeKanji,
				numChars: 0,
				data:     bufFromBits(),
			},
		},
		{
			name:    "test with signal data",
			data:    "ꘞ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KanjiSegment(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("segment constructor error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assertSegment(t, tt.wantData, got)
			}
		})
	}
}

func TestSimpleSegments(t *testing.T) {
	cases := []struct {
		name         string
		text         string
		wantErr      bool
		wantSegments []Segment
	}{
		{
			name:    "test with numeric text",
			text:    "314159265358979323846264338327950288419716939937510",
			wantErr: false,
			wantSegments: []Segment{{
				mode:     ModeNumeric,
				numChars: 51,
				data: bufFromBits(false, true, false, false, true, true, true, false, true, false, false, false, true,
					false, false, true, true, true, true, true, false, true, false, false, false, false, true, false, false,
					true, false, true, false, true, true, false, false, true, true, false, true, true, true, true, false,
					true, false, false, true, true, false, true, false, true, false, false, false, false, true, true, true,
					true, false, true, false, false, true, true, true, false, false, true, false, false, false, false, true,
					false, false, false, false, true, false, true, false, true, false, false, true, false, false, true, false,
					true, false, false, false, true, true, true, true, true, true, false, true, true, false, true, true,
					false, false, true, false, false, true, false, false, false, false, false, false, true, true, false,
					true, false, false, false, true, true, true, false, true, true, false, false, true, true, false, false,
					true, true, true, false, true, false, true, false, true, true, true, true, true, false, true, false, true,
					false, false, true, false, true, true, true, true, true, true, true, true, false),
			}},
		},
		{
			name: "test with byte text",
			text: "https://www.github.com/piglig",
			wantSegments: []Segment{{
				mode:     ModeByte,
				numChars: 29,
				data: bufFromBits(false, true, true, false, true, false, false, false, false, true, true, true, false, true,
					false, false, false, true, true, true, false, true, false, false, false, true, true, true, false, false,
					false, false, false, true, true, true, false, false, true, true, false, false, true, true, true, false,
					true, false, false, false, true, false, true, true, true, true, false, false, true, false, true, true,
					true, true, false, true, true, true, false, true, true, true, false, true, true, true, false, true, true,
					true, false, true, true, true, false, true, true, true, false, false, true, false, true, true, true,
					false, false, true, true, false, false, true, true, true, false, true, true, false, true, false, false,
					true, false, true, true, true, false, true, false, false, false, true, true, false, true, false, false,
					false, false, true, true, true, false, true, false, true, false, true, true, false, false, false, true,
					false, false, false, true, false, true, true, true, false, false, true, true, false, false, false, true,
					true, false, true, true, false, true, true, true, true, false, true, true, false, true, true, false, true,
					false, false, true, false, true, true, true, true, false, true, true, true, false, false, false, false,
					false, true, true, false, true, false, false, true, false, true, true, false, false, true, true, true,
					false, true, true, false, true, true, false, false, false, true, true, false, true, false, false, true,
					false, true, true, false, false, true, true, true),
			}},
		},
		{
			name: "test with alphanumeric text",
			text: "DOLLAR-AMOUNT:$39.87 PERCENTAGE:100.00% OPERATIONS:+-*/",
			wantSegments: []Segment{{
				mode:     ModeAlphanumeric,
				numChars: 55,
				data: bufFromBits(false, true, false, false, true, true, false, false, false, false, true, false, true,
					true, true, true, false, false, false, true, true, false, false, false, true, true, true, false, true,
					true, true, false, true, true, true, true, false, false, true, true, true, true, true, true, false, true,
					true, true, true, true, true, false, true, true, false, true, false, true, false, true, false, true,
					true, true, false, true, true, false, true, false, true, false, false, false, true, false, true, true,
					true, false, true, false, false, false, false, true, false, false, false, false, true, true, false, true,
					true, true, true, true, true, false, false, true, false, true, true, false, true, true, true, true, true,
					true, false, false, true, true, false, true, true, false, true, false, true, false, true, false, false,
					true, false, false, false, true, false, true, false, false, false, true, false, true, false, true, false,
					true, false, false, false, false, true, false, true, false, false, false, false, false, true, true, true,
					false, true, false, false, true, false, false, true, false, true, false, true, false, false, false, true,
					false, false, false, false, false, false, true, false, true, true, false, true, false, false, false, false,
					false, true, false, true, false, true, false, false, false, false, false, false, false, false, false,
					false, false, false, true, true, false, true, true, false, true, false, false, true, false, true, false,
					false, false, true, false, true, false, false, false, true, false, true, false, true, false, false, true,
					false, false, false, true, false, false, true, true, true, false, true, true, true, true, true, false,
					true, true, false, true, false, false, false, false, true, false, true, false, false, false, false, true,
					false, false, true, true, true, true, true, true, true, true, true, false, false, true, false, false,
					true, true, true, false, true, false, true, true, true, false, false, true, false, true, false, true, true,
				),
			}},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := simpleSegments(tt.text)
			if len(got) != len(tt.wantSegments) {
				t.Fatalf("got %d segments, want %d", len(got), len(tt.wantSegments))
			}
			for i := range got {
				assertSegment(t, tt.wantSegments[i], got[i])
			}
		})
	}
}

func TestECISegment(t *testing.T) {
	cases := []struct {
		name        string
		val         int
		wantErr     bool
		wantSegment Segment
	}{
		{
			name:    "test with negative value",
			val:     -1,
			wantErr: true,
		},
		{
			name:    "test with outside value",
			val:     1e6 + 1,
			wantErr: true,
		},
		{
			name:    "test with 100 value",
			val:     100,
			wantErr: false,
			wantSegment: Segment{
				mode:     ModeECI,
				numChars: 0,
				data:     bufFromBits(false, true, true, false, false, true, false, false),
			},
		},
		{
			name:    "test with 1000 value",
			val:     1000,
			wantErr: false,
			wantSegment: Segment{
				mode:     ModeECI,
				numChars: 0,
				data:     bufFromBits(true, false, false, false, false, false, true, true, true, true, true, false, true, false, false, false),
			},
		},
		{
			name:    "test with 99999 value",
			val:     99999,
			wantErr: false,
			wantSegment: Segment{
				mode:     ModeECI,
				numChars: 0,
				data:     bufFromBits(true, true, false, false, false, false, false, true, true, false, false, false, false, true, true, false, true, false, false, true, true, true, true, true),
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ECISegment(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("segment constructor error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				assertSegment(t, tt.wantSegment, got)
			}
		})
	}
}

// assertSegment compares segments by mode, character count and bit content.
func assertSegment(t *testing.T, want, got Segment) {
	t.Helper()
	if want.mode != got.mode || want.numChars != got.numChars {
		t.Errorf("segment = {%v, %d chars}, want {%v, %d chars}", got.mode, got.numChars, want.mode, want.numChars)
	}
	assertEqual(t, bitsOf(&want.data), bitsOf(&got.data))
}

func TestSimpleSegmentsEmpty(t *testing.T) {
	if segs := simpleSegments(""); len(segs) != 0 {
		t.Fatalf("simpleSegments(\"\") = %d segments, want 0", len(segs))
	}
}

func TestModeString(t *testing.T) {
	for m, want := range map[Mode]string{
		ModeNumeric: "numeric", ModeAlphanumeric: "alphanumeric", ModeByte: "byte",
		ModeKanji: "kanji", ModeECI: "eci", Mode(0x3): "Mode(0x3)",
	} {
		assertEqual(t, want, m.String())
	}
}

func TestModeCharCountBits(t *testing.T) {
	tests := []struct {
		mode Mode
		want [3]int // versions 1-9, 10-26, 27-40
	}{
		{ModeNumeric, [3]int{10, 12, 14}},
		{ModeAlphanumeric, [3]int{9, 11, 13}},
		{ModeByte, [3]int{8, 16, 16}},
		{ModeKanji, [3]int{8, 10, 12}},
		{ModeECI, [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		for i, vers := range [3][2]int{{1, 9}, {10, 26}, {27, 40}} {
			for _, ver := range vers {
				if got := tt.mode.charCountBits(ver); got != tt.want[i] {
					t.Errorf("%v.charCountBits(%d) = %d, want %d", tt.mode, ver, got, tt.want[i])
				}
			}
		}
	}
}

func TestSegmentAccessors(t *testing.T) {
	s, err := KanjiSegment("漢字")
	assertNoError(t, err)
	assertEqual(t, ModeKanji, s.Mode())
	assertEqual(t, 2, s.NumChars())
}

func TestTotalBits(t *testing.T) {
	num, _ := NumericSegment("12345") // 4 + 10 + 17
	eci, _ := ECISegment(eciUTF8)     // 4 + 0 + 8
	assertEqual(t, 31+12, totalBits([]Segment{num, eci}, 1))
	assertEqual(t, 0, totalBits(nil, 1))

	// 256 bytes overflow the 8-bit count indicator of versions 1-9 only.
	big := BytesSegment(make([]byte, 256))
	assertEqual(t, -1, totalBits([]Segment{big}, 9))
	assertEqual(t, 4+16+256*8, totalBits([]Segment{big}, 10))
}
