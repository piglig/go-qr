package go_qr

import (
	"testing"
)

func TestGetNumRawDataModules(t *testing.T) {
	testCases := []struct {
		name    string
		version int
		want    int
	}{
		{
			name:    "version equals to MinVersion",
			version: 1,
			want:    208,
		},
		{
			name:    "version equals to MaxVersion",
			version: 40,
			want:    29648,
		},
		{
			name:    "version between 2 and 6",
			version: 4,
			want:    807,
		},
		{
			name:    "version equal or higher than 7",
			version: 9,
			want:    2336,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := getNumRawDataModules(tc.version)
			if got != tc.want {
				t.Errorf("getNumRawDataModules() = %v, want %v", got, tc.want)
			}
		})
	}
}
