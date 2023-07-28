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

			assert.Equal(t, tt.wantData, got)
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

			assert.Equal(t, tt.wantData, got)
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
			assert.Equal(t, tt.wantData, got)
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
		{
			name:     "test with signal data",
			data:     "ꘞ",
			wantErr:  true,
			wantData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeKanji(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantData, got)
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
			assert.Equal(t, tt.wantData, got)
		})
	}
}

func TestEncodeStandardSegments(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		ecl        Ecc
		wantErr    bool
		wantQrCode *QrCode
	}{
		{
			name:    "test with Byte segments",
			text:    "Hello, world!",
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
			name:    "test with Numeric segments",
			text:    "314159265358979323846264338327950288419716939937510",
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
		{
			name: "test with long text",
			text: "AB3CD6EF9GH2IJ5KL8MN0PQ7RS4TUW1VX6YBZ035LH4EJ9QA8RD2VM6BT5UO1EZK7PX3IY6FN0SJ4DC7HQ2WB5LZ8EP4RO1KD6MG3J" +
				"F2HB5UE7LV2NO6SJ1RD9FA8KC3BP6VS1LZ7HN2XF5DQ8RG4JN0SM7ED2VL6HO1PX9FC3KJZB6HD0SE7LQ3VG8NY1TM4PK9RI2AF6DJ5B",
			ecl:     Low,
			wantErr: false,
			wantQrCode: &QrCode{
				version:              7,
				size:                 45,
				errorCorrectionLevel: Low,
				mask:                 3,
				modules: [][]bool{
					{true, true, true, true, true, true, true, false, true, true, true, false, false, true, false, true, false, true, false, false, false, false, true, false, false, false, true, true, true, false, true, false, true, true, false, false, true, false, true, true, true, true, true, true, true},
					{true, false, false, false, false, false, true, false, false, false, true, false, false, true, false, true, false, false, false, true, false, true, false, false, true, false, false, true, true, false, false, false, true, true, false, true, false, false, true, false, false, false, false, false, true},
					{true, false, true, true, true, false, true, false, true, true, true, true, false, true, false, false, true, false, false, true, true, false, false, false, false, true, false, true, false, false, true, true, true, true, false, true, false, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, true, true, false, true, true, false, false, false, false, true, true, true, true, true, false, false, false, false, true, false, false, true, true, false, false, true, false, true, true, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, true, true, false, true, false, false, false, true, true, false, true, true, true, true, true, true, true, true, false, true, true, true, false, false, false, true, true, true, true, false, true, false, true, true, true, false, true},
					{true, false, false, false, false, false, true, false, false, true, false, false, true, false, true, true, false, true, false, false, true, false, false, false, true, false, true, true, false, false, true, true, false, true, false, false, false, false, true, false, false, false, false, false, true},
					{true, true, true, true, true, true, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, true, true, true, true, true, true},
					{false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, true, false, true, false, false, true, false, false, false, true, false, false, true, false, true, true, false, true, false, true, true, false, false, false, false, false, false, false, false, false},
					{true, true, true, true, false, false, true, false, true, false, false, true, false, false, true, false, true, true, false, true, true, true, true, true, true, false, true, false, true, false, false, false, true, false, false, true, false, true, false, false, true, true, true, false, true},
					{true, true, true, true, true, false, false, false, true, false, false, true, true, false, false, false, true, false, true, false, false, false, true, false, false, true, true, false, false, true, true, true, true, true, false, false, true, false, true, true, true, true, false, true, false},
					{false, false, true, false, true, true, true, true, true, false, true, false, true, true, true, false, true, false, true, true, true, false, false, true, true, false, true, true, true, true, false, false, true, true, true, false, true, true, false, false, true, false, true, false, false},
					{false, true, true, true, true, false, false, false, true, false, false, true, true, true, true, true, false, true, false, true, false, false, true, false, false, true, true, true, true, false, true, true, false, true, true, false, true, true, false, false, false, true, true, true, true},
					{false, false, false, false, true, false, true, false, false, true, true, false, false, false, true, false, true, false, true, true, true, true, false, true, true, false, false, true, false, true, true, false, false, false, true, false, true, false, false, true, true, false, true, true, false},
					{true, true, true, true, false, false, false, false, true, true, false, true, true, true, true, true, true, false, false, true, true, false, false, true, false, true, false, false, true, true, false, true, true, true, false, true, true, false, false, true, true, true, false, true, false},
					{true, true, true, false, false, true, true, true, false, false, false, false, true, true, false, false, true, false, false, false, true, true, false, false, false, true, false, false, true, false, true, false, true, true, false, false, false, true, false, true, false, true, true, true, false},
					{false, false, true, false, true, true, false, true, false, false, false, false, false, true, false, false, false, true, false, true, false, true, false, true, false, false, false, true, false, true, false, false, true, false, false, true, false, false, true, false, true, true, false, true, false},
					{true, true, true, true, true, true, true, false, true, false, true, true, true, false, true, true, false, true, false, true, true, false, true, true, false, false, true, true, true, false, false, true, true, true, false, false, false, true, false, true, false, false, false, true, false},
					{false, false, true, true, true, false, false, true, true, true, true, false, true, false, false, false, true, false, true, true, false, false, false, false, true, false, true, false, true, false, true, true, false, false, false, true, true, true, true, false, true, false, false, true, true},
					{false, false, false, true, false, true, true, true, true, true, false, true, true, true, false, false, false, true, false, true, false, false, true, false, true, false, true, true, true, false, true, true, false, false, false, false, true, true, false, false, false, false, false, false, false},
					{true, false, true, false, true, false, false, true, true, false, false, false, false, true, true, false, false, true, true, true, false, false, true, true, false, false, false, false, true, false, true, true, true, false, false, false, true, false, true, false, false, false, false, true, true},
					{true, true, false, true, true, true, true, true, true, true, false, false, false, true, false, false, false, false, true, true, true, true, true, true, true, true, false, false, true, false, true, false, false, false, true, false, true, true, true, true, true, false, false, true, false},
					{false, false, true, true, true, false, false, false, true, false, true, true, false, false, true, true, true, false, false, false, true, false, false, false, true, false, true, false, false, true, false, false, false, false, false, false, true, false, false, false, true, true, true, true, true},
					{true, false, true, false, true, false, true, false, true, false, false, false, true, true, false, false, false, true, false, true, true, false, true, false, true, false, true, true, true, true, false, false, true, true, true, false, true, false, true, false, true, true, false, true, true},
					{false, false, false, false, true, false, false, false, true, false, false, false, true, true, true, false, true, false, true, false, true, false, false, false, true, false, true, false, true, false, false, true, false, false, false, false, true, false, false, false, true, false, false, true, false},
					{true, true, false, true, true, true, true, true, true, true, true, true, true, true, true, true, false, false, false, true, true, true, true, true, true, true, true, false, false, false, false, true, true, true, false, true, true, true, true, true, true, false, false, false, true},
					{true, false, true, true, true, true, false, true, false, false, true, true, false, false, true, true, true, true, true, true, true, true, false, false, false, false, true, false, true, false, false, true, true, false, false, true, false, false, false, false, false, true, false, true, true},
					{true, false, true, true, false, false, true, false, true, true, true, true, false, true, true, false, false, false, false, false, true, true, false, true, false, false, false, false, true, false, false, true, true, false, false, false, false, false, false, true, true, false, true, false, true},
					{false, true, true, true, true, true, false, true, false, false, true, false, true, true, true, false, true, true, true, false, false, false, true, true, false, true, true, true, false, true, true, false, true, false, false, false, false, true, false, true, false, true, true, false, false},
					{true, false, true, false, true, false, true, false, true, false, false, true, true, false, true, false, true, false, false, false, true, true, true, true, false, true, false, false, true, true, false, true, false, true, true, false, false, true, true, true, true, false, false, true, false},
					{true, true, true, true, true, true, false, true, false, true, true, true, true, true, false, false, true, true, true, true, false, true, false, false, false, true, true, true, true, false, false, false, false, true, true, false, true, false, true, true, false, false, true, false, false},
					{false, false, true, false, false, false, true, true, true, true, true, true, true, false, false, true, false, false, false, true, true, false, true, true, true, false, true, true, false, true, true, false, false, true, false, false, true, true, true, true, false, true, true, false, false},
					{false, false, true, false, true, false, false, true, false, true, true, false, false, true, true, true, true, false, false, true, false, true, true, false, false, true, false, false, true, false, true, false, true, true, false, false, false, false, false, true, true, true, true, true, false},
					{false, false, false, false, false, true, true, false, true, false, false, false, false, true, false, false, true, true, false, true, true, true, false, false, false, false, false, true, true, false, true, true, true, false, false, true, false, true, true, true, false, false, true, false, false},
					{false, true, false, true, true, true, false, false, false, false, false, true, false, true, false, true, false, false, true, false, false, true, true, true, true, false, true, false, true, false, true, true, true, false, false, true, false, true, true, false, true, false, true, true, false},
					{false, false, false, false, true, false, true, false, false, true, false, true, true, false, false, false, false, false, false, false, false, true, false, true, true, true, true, true, false, false, false, false, false, true, false, false, false, false, true, true, false, true, true, false, true},
					{false, true, true, true, true, false, false, false, true, false, true, true, true, true, false, false, true, false, true, true, true, true, false, false, true, false, false, false, false, false, false, false, true, false, false, true, true, false, true, true, true, true, false, false, true},
					{true, false, false, true, true, false, true, false, true, false, true, true, false, false, false, false, false, false, false, false, true, true, true, true, true, false, false, true, false, true, true, false, false, false, false, false, true, true, true, true, true, true, true, true, false},
					{false, false, false, false, false, false, false, false, true, true, false, false, true, true, true, true, false, true, false, true, true, false, false, false, true, false, false, true, true, false, true, false, false, false, true, true, true, false, false, false, true, false, false, false, true},
					{true, true, true, true, true, true, true, false, false, false, true, true, false, false, true, true, false, false, true, true, true, false, true, false, true, false, false, false, true, true, true, false, true, false, false, true, true, false, true, false, true, true, false, true, false},
					{true, false, false, false, false, false, true, false, false, true, true, false, false, false, true, true, false, true, false, false, true, false, false, false, true, true, false, false, false, false, false, false, false, false, true, true, true, false, false, false, true, true, true, false, false},
					{true, false, true, true, true, false, true, false, false, true, true, false, false, true, false, true, true, true, true, true, true, true, true, true, true, true, true, false, true, true, false, false, true, true, false, true, true, true, true, true, true, false, false, false, false},
					{true, false, true, true, true, false, true, false, true, true, true, false, false, true, false, false, false, false, false, true, true, true, false, true, true, false, false, false, true, false, false, true, false, false, true, true, false, true, true, false, false, true, false, true, true},
					{true, false, true, true, true, false, true, false, true, false, false, true, true, true, true, false, true, true, false, true, true, true, true, false, false, false, false, false, false, true, false, true, true, false, true, true, true, false, true, false, true, true, true, true, false},
					{true, false, false, false, false, false, true, false, true, false, false, true, false, true, false, false, false, false, false, false, true, false, false, false, true, false, false, true, true, true, true, true, true, false, true, false, true, false, true, true, true, true, true, false, false},
					{true, true, true, true, true, true, true, false, true, true, false, false, true, false, true, true, false, false, false, true, false, true, false, true, true, false, false, true, true, false, false, true, true, true, true, false, true, true, false, true, false, false, true, true, false},
				},
				isFunction: nil,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			segs, err := MakeSegments(tt.text)
			if err != nil {
				t.Errorf("MakeSegments() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err := EncodeStandardSegments(segs, tt.ecl)
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

func TestMakeEci(t *testing.T) {
	cases := []struct {
		name        string
		val         int
		wantErr     bool
		wantSegment *QrSegment
	}{
		{
			name:        "test with negative value",
			val:         -1,
			wantErr:     true,
			wantSegment: nil,
		},
		{
			name:        "test with outside value",
			val:         1e6 + 1,
			wantErr:     true,
			wantSegment: nil,
		},
		{
			name:    "test with 100 value",
			val:     100,
			wantErr: false,
			wantSegment: &QrSegment{
				mode:     Eci,
				numChars: 0,
				data:     &BitBuffer{false, true, true, false, false, true, false, false},
			},
		},
		{
			name:    "test with 1000 value",
			val:     1000,
			wantErr: false,
			wantSegment: &QrSegment{
				mode:     Eci,
				numChars: 0,
				data:     &BitBuffer{true, false, false, false, false, false, true, true, true, true, true, false, true, false, false, false},
			},
		},
		{
			name:    "test with 99999 value",
			val:     99999,
			wantErr: false,
			wantSegment: &QrSegment{
				mode:     Eci,
				numChars: 0,
				data:     &BitBuffer{true, true, false, false, false, false, false, true, true, false, false, false, false, true, true, false, true, false, false, true, true, true, true, true},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeEci(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeSegments() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.wantSegment, got)
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

func TestQrSegment_GetData(t *testing.T) {
	tests := []struct {
		name     string
		segment  *QrSegment
		wantData *BitBuffer
	}{
		{
			name: "test with normal data",
			segment: &QrSegment{
				data: &BitBuffer{true, true, false},
			},
			wantData: &BitBuffer{true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantData, tt.segment.getData())
		})
	}
}
