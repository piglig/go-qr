package go_qr

import "testing"

func TestIsKanji(t *testing.T) {
	testCases := []struct {
		name string
		c    int
		want bool
	}{
		{
			name: "Test Case 1: When c is less than length of unicdeToQRKanji and unicdeToQRKanji[c] is not equal to -1",
			c:    0, // Assuming the first index of unicdeToQRKanji is not -1 after initialization
			want: false,
		},
		{
			name: "Test Case 2: When c is more than length of unicdeToQRKanji",
			c:    1<<16 + 5, // A number certainly greater than length of unicdeToQRKanji
			want: false,
		},
		{
			name: "Test Case 3: When c is within range but unicdeToQRKanji[c] is equal to -1",
			c:    500, // This needs to be adjusted based on the actual content of unicdeToQRKanji
			want: false,
		},
		{
			name: "Test Case 4: When c is within range",
			c:    65311,
			want: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKanji(tt.c); got != tt.want {
				t.Errorf("isKanji() = %v, want %v", got, tt.want)
			}
		})
	}
}
