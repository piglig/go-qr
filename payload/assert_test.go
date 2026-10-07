package payload

import (
	"strings"
	"testing"
)

func assertEqual(t testing.TB, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("not equal:\nwant: %q\n got: %q", want, got)
	}
}

func assertContains(t testing.TB, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("%q does not contain %q", s, substr)
	}
}

func assertTrue(t testing.TB, v bool, msgAndArgs ...any) {
	t.Helper()
	if !v {
		t.Errorf("expected true %v", msgAndArgs)
	}
}
