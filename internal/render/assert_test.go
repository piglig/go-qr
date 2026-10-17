package render

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// Minimal standard-library assertions. Like testify's assert package, a
// failure is reported with t.Errorf and the test keeps running.

func failf(t testing.TB, msgAndArgs []any, format string, args ...any) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	if len(msgAndArgs) > 0 {
		if f, ok := msgAndArgs[0].(string); ok {
			msg += ": " + fmt.Sprintf(f, msgAndArgs[1:]...)
		}
	}
	t.Error(msg)
}

func isEqual(want, got any) bool {
	if w, ok := want.([]byte); ok {
		if g, ok := got.([]byte); ok {
			return bytes.Equal(w, g)
		}
	}
	return reflect.DeepEqual(want, got)
}

func assertEqual(t testing.TB, want, got any, msgAndArgs ...any) {
	t.Helper()
	if !isEqual(want, got) {
		failf(t, msgAndArgs, "not equal:\nwant: %#v\n got: %#v", want, got)
	}
}

func assertNoError(t testing.TB, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		failf(t, msgAndArgs, "unexpected error: %v", err)
	}
}

func assertTrue(t testing.TB, v bool, msgAndArgs ...any) {
	t.Helper()
	if !v {
		failf(t, msgAndArgs, "expected true")
	}
}

func assertFalse(t testing.TB, v bool, msgAndArgs ...any) {
	t.Helper()
	if v {
		failf(t, msgAndArgs, "expected false")
	}
}

func assertInDelta(t testing.TB, want, got, delta float64, msgAndArgs ...any) {
	t.Helper()
	if math.Abs(want-got) > delta {
		failf(t, msgAndArgs, "%v and %v differ by more than %v", want, got, delta)
	}
}
