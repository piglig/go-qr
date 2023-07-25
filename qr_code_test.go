package go_qr

import (
	"testing"
)

func TestFinderPenaltyCountPatterns(t *testing.T) {
	var tests = []struct {
		name        string
		qr          *QrCode
		runHistory  []int
		expectedRes int
	}{
		{"Test1: Core pattern with sufficient initial and final runs", &QrCode{}, []int{8, 2, 2, 6, 2, 2, 8}, 2},
		{"Test2: Core pattern with insufficient initial run", &QrCode{}, []int{1, 2, 2, 6, 2, 2, 8}, 0},
		{"Test3: Core pattern with insufficient final run", &QrCode{}, []int{8, 2, 2, 6, 2, 2, 1}, 0},
		{"Test4: Non core pattern", &QrCode{}, []int{8, 2, 2, 7, 2, 2, 8}, 0},
		{"Test5: Core pattern with equal initial and final runs", &QrCode{}, []int{2, 2, 2, 6, 2, 2, 2}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.qr.finderPenaltyCountPatterns(tt.runHistory)
			if res != tt.expectedRes {
				t.Errorf("got %d, want %d", res, tt.expectedRes)
			}
		})
	}
}

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
