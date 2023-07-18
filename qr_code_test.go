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
