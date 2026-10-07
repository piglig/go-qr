package qr

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
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

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
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

func assertError(t testing.TB, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		failf(t, msgAndArgs, "expected an error, got nil")
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

func assertNil(t testing.TB, v any, msgAndArgs ...any) {
	t.Helper()
	if !isNil(v) {
		failf(t, msgAndArgs, "expected nil, got %#v", v)
	}
}

func assertNotNil(t testing.TB, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		failf(t, msgAndArgs, "expected non-nil value")
	}
}

func assertLen(t testing.TB, v any, n int, msgAndArgs ...any) {
	t.Helper()
	if got := reflect.ValueOf(v).Len(); got != n {
		failf(t, msgAndArgs, "length is %d, want %d", got, n)
	}
}

func assertNotEmpty(t testing.TB, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) || reflect.ValueOf(v).Len() == 0 {
		failf(t, msgAndArgs, "expected a non-empty value")
	}
}

func assertGreater(t testing.TB, a, b int, msgAndArgs ...any) {
	t.Helper()
	if a <= b {
		failf(t, msgAndArgs, "%d is not greater than %d", a, b)
	}
}

func assertInDelta(t testing.TB, want, got, delta float64, msgAndArgs ...any) {
	t.Helper()
	if math.Abs(want-got) > delta {
		failf(t, msgAndArgs, "%v and %v differ by more than %v", want, got, delta)
	}
}

func assertContains(t testing.TB, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if !strings.Contains(s, substr) {
		failf(t, msgAndArgs, "%q does not contain %q", s, substr)
	}
}

// encodeText reproduces v1 EncodeText: one segment in the most compact
// single mode, with ECC boosting. Tests that pin exact symbols use it so the
// expected modules do not depend on the optimal segmenter.
func encodeText(text string, ecc ECC) (*Code, error) {
	return Encode(text, WithECC(ecc), WithSimpleSegmentation())
}
