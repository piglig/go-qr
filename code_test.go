package qr

import "testing"

func TestEncodeSegments(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		ecl      ECC
		wantErr  bool
		wantCode *Code
	}{
		{
			name:    "test with Byte segments",
			text:    "Hello, world!",
			ecl:     ECCLow,
			wantErr: false,
			wantCode: &Code{
				version: MinVersion,
				size:    21,
				ecc:     ECCMedium,
				mask:    2,
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
			},
		},
		{
			name:    "test with Numeric segments",
			text:    "314159265358979323846264338327950288419716939937510",
			ecl:     ECCMedium,
			wantErr: false,
			wantCode: &Code{
				version: 2,
				size:    25,
				ecc:     ECCMedium,
				mask:    3,
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
			},
		},
		{
			name: "test with long text",
			text: "AB3CD6EF9GH2IJ5KL8MN0PQ7RS4TUW1VX6YBZ035LH4EJ9QA8RD2VM6BT5UO1EZK7PX3IY6FN0SJ4DC7HQ2WB5LZ8EP4RO1KD6MG3J" +
				"F2HB5UE7LV2NO6SJ1RD9FA8KC3BP6VS1LZ7HN2XF5DQ8RG4JN0SM7ED2VL6HO1PX9FC3KJZB6HD0SE7LQ3VG8NY1TM4PK9RI2AF6DJ5B",
			ecl:     ECCLow,
			wantErr: false,
			wantCode: &Code{
				version: 7,
				size:    45,
				ecc:     ECCLow,
				mask:    3,
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
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeSegments(simpleSegments(tt.text), WithECC(tt.ecl))
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeSegments() error = %v, wantErr %v", err, tt.wantErr)
			}

			assertEqual(t, tt.wantCode, got)
		})
	}
}

func TestEncodeSimpleEmpty(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		ecl      ECC
		wantErr  bool
		wantCode *Code
	}{
		{
			name:    "test with empty text",
			text:    "",
			ecl:     ECCLow,
			wantErr: false,
			wantCode: &Code{
				version: MinVersion,
				size:    21,
				ecc:     ECCHigh,
				mask:    6,
				modules: [][]bool{
					{true, true, true, true, true, true, true, false, false, true, false, false, false, false, true, true, true, true, true, true, true},
					{true, false, false, false, false, false, true, false, false, false, true, true, false, false, true, false, false, false, false, false, true},
					{true, false, true, true, true, false, true, false, true, true, true, true, true, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, true, true, true, false, true, false, true, false, true, true, true, false, true},
					{true, false, true, true, true, false, true, false, false, true, true, true, false, false, true, false, true, true, true, false, true},
					{true, false, false, false, false, false, true, false, false, true, false, true, true, false, true, false, false, false, false, false, true},
					{true, true, true, true, true, true, true, false, true, false, true, false, true, false, true, true, true, true, true, true, true},
					{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
					{false, false, false, true, true, false, true, true, false, false, false, false, true, false, false, false, false, true, true, false, false},
					{true, true, false, false, false, true, false, true, true, false, false, true, true, false, false, true, true, true, false, true, true},
					{true, true, false, false, false, true, true, true, true, false, false, false, true, true, true, true, false, true, false, false, true},
					{false, true, false, true, true, false, false, false, true, true, true, true, false, false, false, true, true, false, false, true, false},
					{true, false, true, true, false, false, true, false, true, false, false, false, false, false, true, true, true, true, true, true, true},
					{false, false, false, false, false, false, false, false, true, true, false, false, false, true, false, false, false, false, true, true, true},
					{true, true, true, true, true, true, true, false, true, true, true, false, false, true, true, false, false, true, true, false, true},
					{true, false, false, false, false, false, true, false, false, true, true, true, false, true, true, false, false, false, true, false, false},
					{true, false, true, true, true, false, true, false, true, true, false, true, false, false, true, false, true, false, true, true, false},
					{true, false, true, true, true, false, true, false, true, true, false, false, true, false, false, true, true, false, false, false, false},
					{true, false, true, true, true, false, true, false, false, true, false, true, true, false, false, true, true, true, false, true, true},
					{true, false, false, false, false, false, true, false, false, false, false, false, false, true, false, true, false, true, false, true, true},
					{true, true, true, true, true, true, true, false, false, false, true, true, true, false, true, true, true, false, true, true, false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeText(tt.text, tt.ecl)
			if (err != nil) != tt.wantErr {
				t.Errorf("encodeText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assertEqual(t, tt.wantCode, got)
		})
	}
}
