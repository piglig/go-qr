package symbol

import "testing"

func TestCorrectVersion(t *testing.T) {
	for v := 7; v <= MaxVersion; v++ {
		bits := VersionBits(v)
		got, ok := CorrectVersion(bits ^ 0b100000001000000010) // 3 bit errors
		if !ok || got != v {
			t.Fatalf("version %d with 3 errors: got %d, %v", v, got, ok)
		}
	}
	if _, ok := CorrectVersion(0); ok {
		t.Error("all-zero bits should not decode to a version")
	}
}
