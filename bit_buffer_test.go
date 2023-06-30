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

func TestAppendBits(t *testing.T) {
	b := &BitBuffer{}

	expected := []bool{true, false, true}
	err := b.appendBits(5, 3)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !checkEqual(b, expected) {
		t.Errorf("Unexpected BitBuffer state after appendBits: %v, expected: %v", *b, expected)
	}

	err = b.appendBits(2, 2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected = []bool{true, false, true, true, false}
	if !checkEqual(b, expected) {
		t.Errorf("Unexpected BitBuffer state after appendBits: %v, expected: %v", *b, expected)
	}

	err = b.appendBits(1000, 32)
	if err == nil {
		t.Errorf("Expected error: %v", err)
	}

	expected = []bool{true, false, true, true, false}
	if !checkEqual(b, expected) {
		t.Errorf("Unexpected BitBuffer state after appendBits: %v, expected: %v", *b, expected)
	}
}

func checkEqual(b *BitBuffer, expected []bool) bool {
	if b.len() != len(expected) {
		return false
	}

	for i := range expected {
		if b.getBit(i) != expected[i] {
			return false
		}
	}
	return true
}
