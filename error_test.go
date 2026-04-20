package go_qr

import (
	"errors"
	"testing"
)

func TestDataTooLongException_Error(t *testing.T) {
	testCases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "Case 1: Normal Message",
			message: "Data is too long.",
			want:    "Data is too long.",
		},
		{
			name:    "Case 2: Empty Message falls back to sentinel",
			message: "",
			want:    ErrDataTooLong.Error(),
		},
		{
			name:    "Case 3: Longer Message",
			message: "It appears that the data you've entered exceeds our current limit. Please try again with shorter data.",
			want:    "It appears that the data you've entered exceeds our current limit. Please try again with shorter data.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exception := &DataTooLongException{Msg: tc.message}
			if exception.Error() != tc.want {
				t.Errorf("Expected message '%v', but got '%v'", tc.want, exception.Error())
			}
			if !errors.Is(exception, ErrDataTooLong) {
				t.Errorf("DataTooLongException should match ErrDataTooLong via errors.Is")
			}
		})
	}
}
