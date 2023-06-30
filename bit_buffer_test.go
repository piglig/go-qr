package go_qr

import (
	"reflect"
	"testing"
)

var M = 100000

func TestEquivalence(t *testing.T) {
	s := BitBuffer{}
	for j := 2; j < M; j += 13 {
		s.set(j, true)
		if !s.getBit(j) {
			t.Errorf("Expected bit %d to be set %t, but it is not", j, true)
		}
	}

	for j := 1; j < M; j += 5 {
		s.set(j, false)
		if s.getBit(j) {
			t.Errorf("Expected bit %d to be set %t, but it is not", j, false)
		}
	}
}

func TestClone(t *testing.T) {
	a := &BitBuffer{}
	for j := 2; j < M; j += 13 {
		a.set(j, true)
	}

	for j := 1; j < M; j += 5 {
		a.set(j, false)
	}

	b := a.clone()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Expected BitBuffer a and b should be the same")
	}
}
