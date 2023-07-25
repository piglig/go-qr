package go_qr

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMakeAlphanumeric(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantErr  bool
		wantData *QrSegment
	}{
		{
			name:    "test with a digit",
			data:    "0",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Alphanumeric,
				numChars: 1,
				data:     &BitBuffer{false, false, false, false, false, false},
			},
		},
		{
			name:    "test with normal digits",
			data:    "123456",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Alphanumeric,
				numChars: 6,
				data: &BitBuffer{false, false, false, false, false, true, false, true, true, true, true, false, false,
					false, true, false, false, false, true, false, true, true, false, false, false, true, true, true, false,
					false, true, true, true},
			},
		},
		{
			name:    "test with empty data",
			data:    "",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Alphanumeric,
				numChars: 0,
				data:     &BitBuffer{},
			},
		},
		{
			name:     "test with a lower case letter",
			data:     "a",
			wantErr:  true,
			wantData: nil,
		},
		{
			name:    "test with a uppercase letter",
			data:    "A",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Alphanumeric,
				numChars: 1,
				data:     &BitBuffer{false, false, true, false, true, false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeAlphanumeric(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeAlphanumeric() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, got, tt.wantData)
		})
	}
}

func TestMakeNumeric(t *testing.T) {
	tests := []struct {
		name     string
		digits   string
		wantErr  bool
		wantData *QrSegment
	}{
		{
			name:    "test with normal digits",
			digits:  "314159265358979323846264338327950288419716939937510",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Numeric,
				numChars: 51,
				data: &BitBuffer{false, true, false, false, true, true, true, false, true, false, false, false, true,
					false, false, true, true, true, true, true, false, true, false, false, false, false, true, false, false,
					true, false, true, false, true, true, false, false, true, true, false, true, true, true, true, false,
					true, false, false, true, true, false, true, false, true, false, false, false, false, true, true, true,
					true, false, true, false, false, true, true, true, false, false, true, false, false, false, false, true,
					false, false, false, false, true, false, true, false, true, false, false, true, false, false, true, false,
					true, false, false, false, true, true, true, true, true, true, false, true, true, false, true, true,
					false, false, true, false, false, true, false, false, false, false, false, false, true, true, false,
					true, false, false, false, true, true, true, false, true, true, false, false, true, true, false, false,
					true, true, true, false, true, false, true, false, true, true, true, true, true, false, true, false, true,
					false, false, true, false, true, true, true, true, true, true, true, true, false},
			},
		},
		{
			name:     "test with empty digit",
			digits:   "",
			wantErr:  true,
			wantData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeNumeric(tt.digits)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeNumeric() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, got, tt.wantData)
		})
	}
}

func TestMakeBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantErr  bool
		wantData *QrSegment
	}{
		{
			name:    "test with non-nil data",
			data:    []byte("https://www.github.com/piglig"),
			wantErr: false,
			wantData: &QrSegment{
				mode:     Byte,
				numChars: 29,
				data: &BitBuffer{false, true, true, false, true, false, false, false, false, true, true, true, false, true,
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
					false, true, true, false, false, true, true, true},
			},
		},
		{
			name:     "test with nil data",
			data:     nil,
			wantErr:  true,
			wantData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeBytes(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, got, tt.wantData)
		})
	}
}

func TestMakeKanji(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantErr  bool
		wantData *QrSegment
	}{
		{
			name:    "test with Kanji data",
			data:    "「魔法少女まどか☆マギカ」って、　ИАИ　ｄｅｓｕ　κα？",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Kanji,
				numChars: 29,
				data: &BitBuffer{false, false, false, false, false, false, false, true, true, false, true, false, true, true,
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
					false, false, false, false, false, false, true, false, false, false},
			},
		},
		{
			name:    "test with nil data",
			data:    "",
			wantErr: false,
			wantData: &QrSegment{
				mode:     Kanji,
				numChars: 0,
				data:     &BitBuffer{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeKanji(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, got, tt.wantData)
		})
	}
}

func TestNewQrSegment(t *testing.T) {
	tests := []struct {
		name     string
		mode     Mode
		numCh    int
		data     *BitBuffer
		wantErr  bool
		wantData *QrSegment
	}{
		{
			name:     "test with one numCh and no BitBuffer",
			mode:     Numeric,
			numCh:    1,
			data:     &BitBuffer{},
			wantErr:  false,
			wantData: &QrSegment{mode: Numeric, numChars: 1, data: &BitBuffer{}},
		},
		{
			name:     "test with one numCh",
			mode:     Numeric,
			numCh:    1,
			data:     &BitBuffer{true},
			wantErr:  false,
			wantData: &QrSegment{mode: Numeric, numChars: 1, data: &BitBuffer{true}},
		},
		{
			name:     "test with positive numCh",
			mode:     Numeric,
			numCh:    10,
			data:     &BitBuffer{true, true, false},
			wantErr:  false,
			wantData: &QrSegment{mode: Numeric, numChars: 10, data: &BitBuffer{true, true, false}},
		},
		{
			name:     "test with positive numCh and no BitBuffer",
			mode:     Numeric,
			numCh:    10,
			data:     &BitBuffer{},
			wantErr:  false,
			wantData: &QrSegment{mode: Numeric, numChars: 10, data: &BitBuffer{}},
		},
		{
			name:     "test with negative numCh",
			mode:     Numeric,
			numCh:    -1,
			data:     &BitBuffer{},
			wantErr:  true,
			wantData: nil,
		}, {
			name:     "test with zero numCh",
			mode:     Numeric,
			numCh:    0,
			data:     &BitBuffer{},
			wantErr:  false,
			wantData: &QrSegment{mode: Numeric, numChars: 0, data: &BitBuffer{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newQrSegment(tt.mode, tt.numCh, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("newQrSegment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, got, tt.wantData)
		})
	}
}

func TestEncodeStandardSegments(t *testing.T) {
	cases := []struct {
		name       string
		segments   []*QrSegment
		ecl        Ecc
		wantErr    bool
		wantQrCode *QrCode
	}{
		{
			name:       "test with nil segments",
			segments:   nil,
			ecl:        Low,
			wantErr:    true,
			wantQrCode: nil,
		},
		{
			name: "test with Byte segments",
			segments: []*QrSegment{{
				mode:     Byte,
				numChars: 13,
				data: &BitBuffer{false, true, false, false, true, false, false, false, false, true, true, false, false,
					true, false, true, false, true, true, false, true, true, false, false, false, true, true, false, true,
					true, false, false, false, true, true, false, true, true, true, true, false, false, true, false, true,
					true, false, false, false, false, true, false, false, false, false, false, false, true, true, true, false,
					true, true, true, false, true, true, false, true, true, true, true, false, true, true, true, false, false,
					true, false, false, true, true, false, true, true, false, false, false, true, true, false, false, true,
					false, false, false, false, true, false, false, false, false, true},
			}},
			ecl:     Low,
			wantErr: false,
			wantQrCode: &QrCode{
				version:              MinVersion,
				size:                 21,
				errorCorrectionLevel: Medium,
				mask:                 2,
				modules: [][]bool{
					{true, true, true, true, true, true, true, false, false, false, false, false, true, false, true, true, true, true, true, true, true},
					{true, false, false, false, false, false, true, false, false, true, false, true, false, false, true, false, false, false, false, false, true},
					{true, false, true, true, true, false, true, false, true, false, true, true, true, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, true, false, false, false, false, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, true, true, false, false, true, false, true, false, true, true, true, false, true},
					{true, false, false, false, false, false, true, false, true, true, true, true, false, false, true, false, false, false, false, false, true},
					{true, true, true, true, true, true, true, false, true, false, true, false, true, false, true, true, true, true, true, true, true},
					{false, false, false, false, false, false, false, false, true, false, true, false, false, false, false, false, false, false, false, false, false},
					{true, false, true, true, true, true, true, false, false, true, true, true, false, false, true, true, true, true, true, false, false},
					{false, false, false, true, true, false, false, true, true, false, false, false, true, true, false, false, true, true, true, false, true},
					{false, false, false, true, false, false, true, false, true, true, true, false, true, true, true, false, false, true, true, true, false},
					{false, true, true, false, false, true, false, true, false, false, true, true, true, true, false, true, false, true, true, false, false},
					{true, true, false, true, true, true, true, false, true, false, false, false, true, false, true, true, false, false, false, false, true},
					{false, false, false, false, false, false, false, false, true, false, false, false, false, true, true, true, true, true, false, false, false},
					{true, true, true, true, true, true, true, false, false, true, true, false, true, true, true, true, false, false, true, true, false},
					{true, false, false, false, false, false, true, false, true, false, true, false, true, true, false, true, false, true, true, true, false},
					{true, false, true, true, true, false, true, false, true, true, false, true, true, true, true, false, true, false, false, true, true},
					{true, false, true, true, true, false, true, false, true, false, true, false, false, false, false, true, true, true, false, false, false},
					{true, false, true, true, true, false, true, false, true, true, true, true, true, false, true, true, false, false, true, false, false},
					{true, false, false, false, false, false, true, false, false, false, true, false, true, true, false, false, true, true, true, false, false},
					{true, true, true, true, true, true, true, false, true, true, false, true, false, false, true, false, true, false, false, true, false},
				},
				isFunction: nil,
			},
		},
		{
			name: "test with Numeric segments",
			segments: []*QrSegment{
				{mode: Numeric, numChars: 51, data: &BitBuffer{
					false, true, false, false, true, true, true, false, true, false, false, false, true,
					false, false, true, true, true, true, true, false, true, false, false, false, false, true, false, false,
					true, false, true, false, true, true, false, false, true, true, false, true, true, true, true, false,
					true, false, false, true, true, false, true, false, true, false, false, false, false, true, true, true,
					true, false, true, false, false, true, true, true, false, false, true, false, false, false, false, true,
					false, false, false, false, true, false, true, false, true, false, false, true, false, false, true, false,
					true, false, false, false, true, true, true, true, true, true, false, true, true, false, true, true,
					false, false, true, false, false, true, false, false, false, false, false, false, true, true, false,
					true, false, false, false, true, true, true, false, true, true, false, false, true, true, false, false,
					true, true, true, false, true, false, true, false, true, true, true, true, true, false, true, false, true,
					false, false, true, false, true, true, true, true, true, true, true, true, false},
				}},
			ecl:     Medium,
			wantErr: false,
			wantQrCode: &QrCode{
				version:              2,
				size:                 25,
				errorCorrectionLevel: Medium,
				mask:                 3,
				modules: [][]bool{
					{true, true, true, true, true, true, true, false, true, false, false, false, false, false, true, false, false, false, true, true, true, true, true, true, true},
					{true, false, false, false, false, false, true, false, true, false, true, false, true, true, true, false, true, false, true, false, false, false, false, false, true},
					{true, false, true, true, true, false, true, false, false, false, true, false, false, false, true, true, true, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, true, true, false, true, true, true, false, false, true, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, false, true, false, false, true, true, true, true, false, false, true, false, true, true, true, false, true},
					{true, false, false, false, false, false, true, false, false, true, false, false, true, false, false, false, true, false, true, false, false, false, false, false, true},
					{true, true, true, true, true, true, true, false, true, false, true, false, true, false, true, false, true, false, true, true, true, true, true, true, true},
					{false, false, false, false, false, false, false, false, true, false, false, true, true, true, false, true, false, false, false, false, false, false, false, false, false},
					{true, false, true, true, false, true, true, true, false, true, false, true, false, false, true, true, false, false, true, false, false, true, false, true, true},
					{false, false, false, false, true, true, false, false, true, false, true, true, true, true, false, true, false, false, true, true, false, false, true, true, false},
					{true, true, true, false, false, true, true, false, false, true, true, false, true, true, true, false, false, true, false, false, false, true, false, false, false},
					{true, false, false, false, true, false, false, true, false, false, false, false, false, true, false, false, true, false, true, false, true, false, true, false, true},
					{false, true, false, true, true, false, true, true, false, true, false, true, false, false, true, false, false, false, true, false, false, true, false, false, true},
					{false, true, false, false, true, false, false, false, true, true, true, true, false, true, false, false, true, false, false, true, true, false, true, true, true},
					{false, true, true, true, true, false, true, false, true, false, true, false, true, false, true, false, true, true, false, false, true, true, true, false, true},
					{true, false, false, true, false, false, false, true, true, true, true, false, true, true, true, false, true, true, true, true, true, false, false, true, false},
					{false, false, true, true, false, false, true, true, true, true, true, true, false, true, false, false, true, true, true, true, true, true, false, true, false},
					{false, false, false, false, false, false, false, false, true, false, true, false, true, true, false, true, true, false, false, false, true, false, false, true, false},
					{true, true, true, true, true, true, true, false, true, true, true, true, true, false, false, false, true, false, true, false, true, false, false, true, false},
					{true, false, false, false, false, false, true, false, true, true, true, false, true, false, true, false, true, false, false, false, true, false, true, true, false},
					{true, false, true, true, true, false, true, false, false, true, false, true, true, true, false, true, true, true, true, true, true, true, false, true, true},
					{true, false, true, true, true, false, true, false, true, true, false, false, true, true, false, false, true, false, true, false, false, false, true, false, true},
					{true, false, true, true, true, false, true, false, true, true, false, true, true, true, false, true, true, true, true, false, true, true, false, true, false},
					{true, false, false, false, false, false, true, false, false, true, false, false, false, false, false, true, true, true, false, false, false, false, true, true, false},
					{true, true, true, true, true, true, true, false, true, false, false, false, true, false, true, true, false, true, true, true, false, false, true, false, true},
				},
				isFunction: nil,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeStandardSegments(tt.segments, tt.ecl)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeSegments() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.wantQrCode, got)
		})
	}
}

func TestMakeSegments(t *testing.T) {
	cases := []struct {
		name         string
		text         string
		wantErr      bool
		wantSegments []*QrSegment
	}{
		{
			name:         "test with empty text",
			text:         "",
			wantErr:      false,
			wantSegments: []*QrSegment{},
		},
		{
			name:    "test with numeric text",
			text:    "314159265358979323846264338327950288419716939937510",
			wantErr: false,
			wantSegments: []*QrSegment{{
				mode:     Numeric,
				numChars: 51,
				data: &BitBuffer{false, true, false, false, true, true, true, false, true, false, false, false, true,
					false, false, true, true, true, true, true, false, true, false, false, false, false, true, false, false,
					true, false, true, false, true, true, false, false, true, true, false, true, true, true, true, false,
					true, false, false, true, true, false, true, false, true, false, false, false, false, true, true, true,
					true, false, true, false, false, true, true, true, false, false, true, false, false, false, false, true,
					false, false, false, false, true, false, true, false, true, false, false, true, false, false, true, false,
					true, false, false, false, true, true, true, true, true, true, false, true, true, false, true, true,
					false, false, true, false, false, true, false, false, false, false, false, false, true, true, false,
					true, false, false, false, true, true, true, false, true, true, false, false, true, true, false, false,
					true, true, true, false, true, false, true, false, true, true, true, true, true, false, true, false, true,
					false, false, true, false, true, true, true, true, true, true, true, true, false},
			}},
		},
		{
			name: "test with byte text",
			text: "https://www.github.com/piglig",
			wantSegments: []*QrSegment{{
				mode:     Byte,
				numChars: 29,
				data: &BitBuffer{false, true, true, false, true, false, false, false, false, true, true, true, false, true,
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
					false, true, true, false, false, true, true, true},
			}},
		},
		{
			name: "test with alphanumeric text",
			text: "DOLLAR-AMOUNT:$39.87 PERCENTAGE:100.00% OPERATIONS:+-*/",
			wantSegments: []*QrSegment{{
				mode:     Alphanumeric,
				numChars: 55,
				data: &BitBuffer{false, true, false, false, true, true, false, false, false, false, true, false, true,
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
				},
			}},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeSegments(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeSegments() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.wantSegments, got)
		})
	}
}

func TestIsModes(t *testing.T) {
	tests := []struct {
		name      string
		mode      Mode
		isNumeric bool
		isAlpha   bool
		isByte    bool
		isKanji   bool
		isEci     bool
	}{
		{
			"Numeric mode",
			Numeric,
			true,
			false,
			false,
			false,
			false,
		},
		{
			"Alphanumeric mode",
			Alphanumeric,
			false,
			true,
			false,
			false,
			false,
		},
		{
			"Byte mode",
			Byte,
			false,
			false,
			true,
			false,
			false,
		},
		{
			"Eci mode",
			Eci,
			false,
			false,
			false,
			false,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.isNumeric(); got != tt.isNumeric {
				t.Errorf("Mode.isNumeric() = %v, want %v", got, tt.isNumeric)
			}
			if got := tt.mode.isAlphanumeric(); got != tt.isAlpha {
				t.Errorf("Mode.isAlphanumeric() = %v, want %v", got, tt.isAlpha)
			}
			if got := tt.mode.isByte(); got != tt.isByte {
				t.Errorf("Mode.isByte() = %v, want %v", got, tt.isByte)
			}
			if got := tt.mode.isKanji(); got != tt.isKanji {
				t.Errorf("Mode.isKanji() = %v, want %v", got, tt.isKanji)
			}
			if got := tt.mode.isEci(); got != tt.isEci {
				t.Errorf("Mode.isEci() = %v, want %v", got, tt.isEci)
			}
		})
	}
}
