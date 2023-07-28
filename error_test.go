package go_qr

import "testing"

func TestDataTooLongException_Error(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{
			name:    "Case 1: Normal Message",
			message: "Data is too long.",
		},
		{
			name:    "Case 2: Empty Message",
			message: "",
		},
		{
			name:    "Case 3: Longer Message",
			message: "It appears that the data you've entered exceeds our current limit. Please try again with shorter data.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exception := &DataTooLongException{Msg: tc.message}
			if exception.Error() != tc.message {
				t.Errorf("Expected message '%v', but got '%v'", tc.message, exception.Error())
			}
		})
	}
}
