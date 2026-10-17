package detect

import (
	"bytes"
	"fmt"
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

func assertNotNil(t testing.TB, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		failf(t, msgAndArgs, "expected non-nil value")
	}
}
