package go_qr

import "testing"

func TestReedSolomonMultiply(t *testing.T) {
	testCases := []struct {
		x        int
		y        int
		expected int
	}{
		{0, 0, 0},
		{1, 1, 1},
		{2, 3, 6},
		{4, 4, 16},
		{5, 8, 40},
		{9, 7, 63},
	}

	for _, tc := range testCases {
		result := reedSolomonMultiply(tc.x, tc.y)
		if result != tc.expected {
			t.Errorf("For x: %d, y: %d, Expected: %d, but got: %d", tc.x, tc.y, tc.expected, result)
		}
	}
}

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
