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

func TestIsAlphanumeric(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"HELLO WORLD", true},       // contains uppercase letters and a space
		{"12345", true},             // contains numbers
		{"$%*+-./:", true},          // contains valid special characters
		{"hello world", false},      // contains lowercase letters
		{"_NotValid", false},        // contains underscore which is not a valid character
		{"123abc", false},           // contains lowercase letters
		{"Special!@#", false},       // contains special characters that are not allowed
		{"Mixed123CASE$", false},    // mixture of digits, uppercase letters and a valid special character
		{"://www.apple.com", false}, // contains double slashes and lower case letters
	}
	for _, c := range cases {
		got := isAlphanumeric(c.in)
		if got != c.want {
			t.Errorf("isisAlphanumeric(%q) == %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"12345", true},         // contains only numbers
		{"ABCDE", false},        // contains no numbers
		{"abc123", false},       // contains numbers and lowercase letters
		{"ABC123", false},       // contains numbers and uppercase letters
		{"Special!@#1", false},  // contains special characters and a number
		{"Special!@#", false},   // contains special characters, but no number
		{" ", false},            // contains only a whitespace character
		{"Mixed123CASE", false}, // mixture of digits, uppercase and lower case letters
		{"1.23", false},         // contains numbers and a dot
		{"0", true},             // contains a number
	}
	for _, c := range cases {
		got := isNumeric(c.in)
		if got != c.want {
			t.Errorf("isNumeric(%q) == %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNumCharCountBits(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		ver  int
		want int
	}{
		{
			"Numeric mode, version 0",
			Numeric,
			0,
			10,
		},
		{
			"Alphanumeric mode, version 20",
			Alphanumeric,
			20,
			11,
		},
		{
			"Byte mode, version 25",
			Byte,
			25,
			16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.numCharCountBits(tt.ver); got != tt.want {
				t.Errorf("Mode.numCharCountBits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetModeBits(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want int
	}{
		{
			"Numeric mode bits",
			Numeric,
			0x1,
		},
		{
			"Alphanumeric mode bits",
			Alphanumeric,
			0x2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.getModeBits(); got != tt.want {
				t.Errorf("Mode.getModeBits() = %v, want %v", got, tt.want)
			}
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
