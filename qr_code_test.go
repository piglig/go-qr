package go_qr

import (
	"reflect"
	"testing"
)

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

func TestGetNumRawDataModules(t *testing.T) {
	testCases := []struct {
		name    string
		version int
		want    int
		wantErr bool
	}{
		{
			name:    "version equals to MinVersion",
			version: 1,
			want:    208,
			wantErr: false,
		},
		{
			name:    "version equals to MaxVersion",
			version: 40,
			want:    29648,
			wantErr: false,
		},
		{
			name:    "version between 2 and 6",
			version: 4,
			want:    807,
			wantErr: false,
		},
		{
			name:    "version equal or higher than 7",
			version: 9,
			want:    2336,
			wantErr: false,
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

func TestGetEccCodeWordsPerBlock(t *testing.T) {
	expected := [][]int8{
		{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
		{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28},
		{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
		{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	}
	if !reflect.DeepEqual(getEccCodeWordsPerBlock(), expected) {
		t.Errorf("getEccCodeWordsPerBlock() did not return expected value")
	}
}

func TestGetNumErrorCorrectionBlocks(t *testing.T) {
	expected := [][]int8{
		{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},
		{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},
		{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},
		{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81},
	}
	if !reflect.DeepEqual(getNumErrorCorrectionBlocks(), expected) {
		t.Errorf("getNumErrorCorrectionBlocks() did not return expected value")
	}
}

func TestFinderPenaltyAddHistory(t *testing.T) {
	tests := []struct {
		q              *QrCode
		currentRunLen  int
		runHistory     []int
		expectedOutput []int
	}{
		{
			q:              &QrCode{size: 5},
			currentRunLen:  10,
			runHistory:     []int{0, 2, 3, 4},
			expectedOutput: []int{15, 0, 2, 3},
		},
		{
			q:              &QrCode{size: 8},
			currentRunLen:  12,
			runHistory:     []int{1, 2, 3, 4},
			expectedOutput: []int{12, 1, 2, 3},
		},
		{
			q:              &QrCode{size: 6},
			currentRunLen:  7,
			runHistory:     []int{0, 0, 0, 0},
			expectedOutput: []int{13, 0, 0, 0},
		},
	}

	for _, test := range tests {
		test.q.finderPenaltyAddHistory(test.currentRunLen, test.runHistory)
		if !reflect.DeepEqual(test.runHistory, test.expectedOutput) {
			t.Errorf("Expected %v, but got %v", test.expectedOutput, test.runHistory)
		}
	}
}
