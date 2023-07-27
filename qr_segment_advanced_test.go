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
