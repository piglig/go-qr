package go_qr

import (
	"bytes"
	"errors"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type badWriter struct{}

func (bw *badWriter) Write(p []byte) (n int, err error) {
	return -1, errors.New("sorry, all I do is fail")
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

func TestEncodeText(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		ecl        Ecc
		wantErr    bool
		wantQrCode *QrCode
	}{
		{
			name:    "test with empty text",
			text:    "",
			ecl:     Low,
			wantErr: false,
			wantQrCode: &QrCode{
				version:              MinVersion,
				size:                 21,
				errorCorrectionLevel: High,
				mask:                 6,
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
				isFunction: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeText(tt.text, tt.ecl)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.wantQrCode, got)
		})
	}
}

func TestQrCode_PNG(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)
	tests := []struct {
		text    string
		wantErr bool
		ecl     Ecc
		dest    string
		config  *QrCodeImgConfig
	}{
		{
			text:    "Hello, world!",
			wantErr: false,
			ecl:     Low,
			dest:    "hello-world-QR.png",
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "",
			wantErr: false,
			ecl:     Low,
			dest:    "empty-QR.png",
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "こんにちwa、世界！ αβγδ",
			wantErr: false,
			ecl:     Quartile,
			dest:    "unicode-QR.png",
			config:  NewQrCodeImgConfig(10, 3),
		},
		{
			text:    "aabbcc",
			wantErr: true,
			ecl:     Quartile,
			dest:    "aabbcc-QR.png",
			config:  NewQrCodeImgConfig(-10, -3),
		},
		{
			text:    "non-existent path",
			wantErr: true,
			ecl:     Low,
			dest:    "../../not/existing.png",
			config:  NewQrCodeImgConfig(10, 3),
		},
	}

	for _, tt := range tests {
		qr, err := EncodeText(tt.text, tt.ecl)
		if err != nil {
			t.Errorf("EncodeText() error = %v", err)
			return
		}

		dest := filepath.Join(tempDir, tt.dest)
		err = qr.PNG(tt.config, dest)
		if (err != nil) != tt.wantErr {
			t.Errorf("TestQrCode_PNG() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

		if err == nil {
			_, err = os.Stat(dest)
			if err != nil {
				t.Errorf("TestQrCode_PNG() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		}
	}
}

func TestQrCode_WriteAsPNG(t *testing.T) {
	tests := []struct {
		text    string
		wantErr bool
		ecl     Ecc
		dest    io.Writer
		config  *QrCodeImgConfig
	}{
		{
			text:    "Hello, world!",
			wantErr: false,
			ecl:     Low,
			dest:    &bytes.Buffer{},
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "",
			wantErr: false,
			ecl:     Low,
			dest:    &bytes.Buffer{},
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "こんにちwa、世界！ αβγδ",
			wantErr: false,
			ecl:     Quartile,
			dest:    &bytes.Buffer{},
			config:  NewQrCodeImgConfig(10, 3),
		},
		{
			text:    "Negative scale",
			wantErr: true,
			ecl:     Quartile,
			dest:    nil,
			config:  NewQrCodeImgConfig(-10, 3),
		},
		{
			text:    "Negative border",
			wantErr: true,
			ecl:     Quartile,
			dest:    nil,
			config:  NewQrCodeImgConfig(10, -3),
		},
		{
			text:    "Too large border",
			wantErr: true,
			ecl:     Quartile,
			dest:    nil,
			config:  NewQrCodeImgConfig(10, math.MaxInt32),
		},
		{
			text:    "Fail on write",
			wantErr: true,
			ecl:     Quartile,
			dest:    &badWriter{},
			config:  NewQrCodeImgConfig(10, 3),
		},
	}

	for _, tt := range tests {
		qr, err := EncodeText(tt.text, tt.ecl)
		if err != nil {
			t.Errorf("EncodeText() error = %v", err)
			return
		}

		err = qr.WriteAsPNG(tt.config, tt.dest)
		if (err != nil) != tt.wantErr {
			t.Errorf("TestQrCode_WriteAsPNG() error = %v, wantErr %v", err, tt.wantErr)
			return
		}
	}
}

func TestNewQrCodeImgConfig(t *testing.T) {
	colorSetterFunc := func(config *QrCodeImgConfig, light, dark color.Color) {
		if light != nil {
			config.SetLight(light)
		}

		if dark != nil {
			config.SetDark(dark)
		}
	}

	tests := []struct {
		name        string
		scale       int
		border      int
		light, dark color.Color
		options     []func(config *QrCodeImgConfig)
		colorSetter func(config *QrCodeImgConfig, light, dark color.Color)
		want        *QrCodeImgConfig
	}{
		{
			name:        "Default colors",
			scale:       5,
			border:      10,
			light:       color.White,
			dark:        color.Black,
			options:     nil,
			colorSetter: colorSetterFunc,
			want: &QrCodeImgConfig{
				scale:  5,
				border: 10,
				light:  color.White,
				dark:   color.Black,
				options: &qrCodeConfig{
					svgXMLHeader: false,
				},
			},
		},
		{
			name:        "Change dark color",
			scale:       5,
			border:      10,
			light:       color.White,
			dark:        color.White,
			colorSetter: colorSetterFunc,
			want: &QrCodeImgConfig{
				scale:  5,
				border: 10,
				light:  color.White,
				dark:   color.White,
				options: &qrCodeConfig{
					svgXMLHeader: false,
				},
			},
		},
		{
			name:        "Change light color",
			scale:       5,
			border:      10,
			light:       color.Black,
			dark:        color.Black,
			colorSetter: colorSetterFunc,
			want: &QrCodeImgConfig{
				scale:  5,
				border: 10,
				light:  color.Black,
				dark:   color.Black,
				options: &qrCodeConfig{
					svgXMLHeader: false,
				},
			},
		},
		{
			name:        "Change light and dark colors",
			scale:       5,
			border:      10,
			light:       color.Black,
			dark:        color.White,
			colorSetter: colorSetterFunc,
			want: &QrCodeImgConfig{
				scale:  5,
				border: 10,
				light:  color.Black,
				dark:   color.White,
				options: &qrCodeConfig{
					svgXMLHeader: false,
				},
			},
		},
		{
			name:        "Valid config with options",
			scale:       5,
			border:      10,
			light:       color.White,
			dark:        color.Black,
			options:     []func(config *QrCodeImgConfig){WithSVGXMLHeader()},
			colorSetter: colorSetterFunc,
			want: &QrCodeImgConfig{
				scale:  5,
				border: 10,
				light:  color.White,
				dark:   color.Black,
				options: &qrCodeConfig{
					svgXMLHeader: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewQrCodeImgConfig(tt.scale, tt.border, tt.options...)
			tt.colorSetter(got, tt.light, tt.dark)
			if got.scale != tt.want.scale {
				t.Errorf("scale = %v, want %v", got, &tt.want)
			}

			if got.border != tt.want.border {
				t.Errorf("border = %v, want %v", got, &tt.want)
			}

			if got.Light() != tt.want.Light() {
				t.Errorf("light color = %v, want %v", got, &tt.want)
			}

			if got.Dark() != tt.want.Dark() {
				t.Errorf("dark color = %v, want %v", got, &tt.want)
			}
			assert.Equal(t, tt.want.options, got.options)
		})
	}
}

func TestQrCode_SVG(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)
	tests := []struct {
		text    string
		wantErr bool
		ecl     Ecc
		dest    string
		config  *QrCodeImgConfig
	}{
		{
			text:    "Hello, world!",
			wantErr: false,
			ecl:     Low,
			dest:    "hello-world-QR.svg",
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "Hello, world!",
			wantErr: false,
			ecl:     Low,
			dest:    "hello-world-QR-with-svg-xml.svg",
			config:  NewQrCodeImgConfig(10, 4, WithSVGXMLHeader()),
		},
		{
			text:    "Hello, world!",
			wantErr: false,
			ecl:     Low,
			dest:    "hello-world-QR-with-optimal-svg-xml.svg",
			config:  NewQrCodeImgConfig(10, 4, WithSVGXMLHeader(), WithOptimalSVG()),
		},
		{
			text:    "",
			wantErr: false,
			ecl:     Low,
			dest:    "empty-QR.svg",
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "こんにちwa、世界！ αβγδ",
			wantErr: false,
			ecl:     Quartile,
			dest:    "unicode-QR.svg",
			config:  NewQrCodeImgConfig(10, 3),
		},
		{
			text:    "aabbcc",
			wantErr: true,
			ecl:     Quartile,
			dest:    "aabbcc-QR.svg",
			config:  NewQrCodeImgConfig(-10, -3),
		},
		{
			text:    "invalid file name",
			wantErr: true,
			ecl:     Low,
			dest:    "test.other",
			config:  NewQrCodeImgConfig(10, 3),
		},
		{
			text:    "non-existent path",
			wantErr: true,
			ecl:     Low,
			dest:    "../../not/existing.svg",
			config:  NewQrCodeImgConfig(10, 3),
		},
	}

	for _, tt := range tests {
		qr, err := EncodeText(tt.text, tt.ecl)
		if err != nil {
			t.Errorf("EncodeText() error = %v", err)
			return
		}

		dest := filepath.Join(tempDir, tt.dest)
		err = qr.SVG(tt.config, dest)
		if (err != nil) != tt.wantErr {
			t.Errorf("TestQrCode_SVG() error = %v, text = %v, wantErr %v", err, tt.text, tt.wantErr)
			return
		}

		if err == nil {
			_, err = os.Stat(dest)
			if err != nil {
				t.Errorf("TestQrCode_SVG() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		}
	}
}

func TestQrCode_WriteAsSVG(t *testing.T) {
	tests := []struct {
		text    string
		wantErr bool
		ecl     Ecc
		dest    io.Writer
		config  *QrCodeImgConfig
	}{
		{
			text:    "Hello, world!",
			wantErr: false,
			ecl:     Low,
			dest:    &bytes.Buffer{},
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "",
			wantErr: false,
			ecl:     Low,
			dest:    &bytes.Buffer{},
			config:  NewQrCodeImgConfig(10, 4),
		},
		{
			text:    "こんにちwa、世界！ αβγδ",
			wantErr: false,
			ecl:     Quartile,
			dest:    &bytes.Buffer{},
			config:  NewQrCodeImgConfig(10, 3),
		},
		{
			text:    "Negative scale",
			wantErr: true,
			ecl:     Quartile,
			dest:    nil,
			config:  NewQrCodeImgConfig(-10, 3),
		},
		{
			text:    "Negative border",
			wantErr: true,
			ecl:     Quartile,
			dest:    nil,
			config:  NewQrCodeImgConfig(10, -3),
		},
		{
			text:    "Fail on write",
			wantErr: true,
			ecl:     Quartile,
			dest:    &badWriter{},
			config:  NewQrCodeImgConfig(10, 3),
		},
	}

	light, dark := "#FFFFFF", "#000000"
	for _, tt := range tests {
		qr, err := EncodeText(tt.text, tt.ecl)
		if err != nil {
			t.Errorf("EncodeText() error = %v", err)
			return
		}

		err = qr.WriteAsSVG(tt.config, tt.dest)
		if (err != nil) != tt.wantErr {
			t.Errorf("TestQrCode_WriteAsSVG() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

		if !tt.wantErr {
			actualSVGString := tt.dest.(*bytes.Buffer).String()
			expectedSVGString := qr.toSVGString(tt.config, light, dark)

			if actualSVGString != expectedSVGString {
				t.Error("TestQrCode_WriteAsSVG() svg string does not match the content of the io.Writer")
				return
			}
		}
	}
}

func BenchmarkToSVGString(b *testing.B) {
	qr, _ := EncodeText("WIFI:S:mYwIfI;T:WPA;P:secret_passwordt;H:false;;", Medium)
	cfg := NewQrCodeImgConfig(10, 4)
	light, dark := "#FFFFFF", "#000000"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qr.toSVGString(cfg, light, dark)
	}
}

func TestToPNGBytes(t *testing.T) {
	qr, err := EncodeText("Hello, world!", Low)
	assert.NoError(t, err)

	t.Run("returns valid PNG bytes", func(t *testing.T) {
		b, err := qr.ToPNGBytes(NewQrCodeImgConfig(10, 4))
		assert.NoError(t, err)
		assert.NotEmpty(t, b)
		// PNG signature: 89 50 4E 47 0D 0A 1A 0A
		assert.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, b[:8])
	})

	t.Run("matches WriteAsPNG output", func(t *testing.T) {
		cfg := NewQrCodeImgConfig(10, 4)
		var buf bytes.Buffer
		assert.NoError(t, qr.WriteAsPNG(cfg, &buf))
		b, err := qr.ToPNGBytes(cfg)
		assert.NoError(t, err)
		assert.Equal(t, buf.Bytes(), b)
	})

	t.Run("rejects invalid config", func(t *testing.T) {
		_, err := qr.ToPNGBytes(NewQrCodeImgConfig(-1, 4))
		assert.Error(t, err)
	})
}

func TestToSVGBytes(t *testing.T) {
	qr, err := EncodeText("Hello, world!", Low)
	assert.NoError(t, err)

	t.Run("returns valid SVG bytes", func(t *testing.T) {
		b, err := qr.ToSVGBytes(NewQrCodeImgConfig(10, 4))
		assert.NoError(t, err)
		assert.Contains(t, string(b), "<svg")
		assert.Contains(t, string(b), "</svg>")
	})

	t.Run("honors optimal option", func(t *testing.T) {
		b, err := qr.ToSVGBytes(NewQrCodeImgConfig(10, 4, WithOptimalSVG()))
		assert.NoError(t, err)
		assert.Contains(t, string(b), "fill-rule=\"evenodd\"")
	})

	t.Run("matches WriteAsSVG output", func(t *testing.T) {
		cfg := NewQrCodeImgConfig(10, 4)
		var buf bytes.Buffer
		assert.NoError(t, qr.WriteAsSVG(cfg, &buf))
		b, err := qr.ToSVGBytes(cfg)
		assert.NoError(t, err)
		assert.Equal(t, buf.Bytes(), b)
	})

	t.Run("rejects invalid config", func(t *testing.T) {
		_, err := qr.ToSVGBytes(NewQrCodeImgConfig(0, 4))
		assert.Error(t, err)
	})
}

func TestToImage(t *testing.T) {
	qr, err := EncodeText("Hello, world!", Low)
	assert.NoError(t, err)

	t.Run("returns image with expected dimensions", func(t *testing.T) {
		img, err := qr.ToImage(NewQrCodeImgConfig(10, 4))
		assert.NoError(t, err)
		expected := (qr.GetSize() + 8) * 10
		assert.Equal(t, expected, img.Bounds().Dx())
		assert.Equal(t, expected, img.Bounds().Dy())
	})

	t.Run("rejects invalid config", func(t *testing.T) {
		_, err := qr.ToImage(NewQrCodeImgConfig(-1, 4))
		assert.Error(t, err)
	})
}

func BenchmarkToOptimalSVGString(b *testing.B) {
	text := "WIFI:S:mYwIfI;T:WPA;P:secret_passwordt;H:false;;"
	ecl := Medium
	light, dark := "#FFFFFF", "#000000"
	for i := 0; i < b.N; i++ {
		qr, _ := EncodeText(text, ecl)
		qr.toSvgOptimizedString(NewQrCodeImgConfig(10, 4), light, dark)
	}
}
