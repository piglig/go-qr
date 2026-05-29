package go_qr

import (
	"errors"
	"strings"
	"testing"
)

func TestEncodeText_DataTooLongWrapsSentinel(t *testing.T) {
	// 5000 alphanumeric chars exceed the capacity of every version/ECC level.
	long := strings.Repeat("A", 5000)
	_, err := EncodeText(long, High)
	if err == nil {
		t.Fatal("expected an error for over-long data")
	}
	if !errors.Is(err, ErrDataTooLong) {
		t.Errorf("expected errors.Is(err, ErrDataTooLong), got %v", err)
	}
}
