package go_qr

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
					data: &BitBuffer{
						false, true, false, false, true, false, false, false, false, true, true, false, false, true, false,
						true, false, true, true, false, true, true, false, false, false, true, true, false, true, true, false,
						false, false, true, true, false, true, true, true, true, false, false, true, false, true, true, false,
						false, false, false, true, false, false, false, false, false, false, true, false, true, false, true,
						true, true, false, true, true, false, true, true, true, true, false, true, true, true, false, false,
						true, false, false, true, true, false, true, true, false, false, false, true, true, false, false, true,
						false, false, false, false, true, false, false, false, false, true,
					},
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
					data: &BitBuffer{
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
					},
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
					data: &BitBuffer{
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
					},
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
					data: &BitBuffer{
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
					},
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
