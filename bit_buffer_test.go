package go_qr

import "testing"

var M = 100000

func TestEquivalence(t *testing.T) {
	s := BitBuffer{}
	for j := 2; j < M; j += 13 {
		s.Set(j, true)
		if !s.GetBit(j) {
			t.Errorf("Expected bit %d to be set %t, but it is not", j, true)
		}
	}

	for j := 1; j < M; j += 5 {
		s.Set(j, false)
		if s.GetBit(j) {
			t.Errorf("Expected bit %d to be set %t, but it is not", j, false)
		}
	}
}
