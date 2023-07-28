package go_qr

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var M = 100000

func TestEquivalence(t *testing.T) {
	s := BitBuffer{}

	if s.getBit(1) {
		t.Errorf("Expected bit %d not to be set %t, but it is not", 1, true)
	}

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
	assert.Equal(t, a, b)
}

func TestAppendBits(t *testing.T) {

	tests := []struct {
		name     string
		original BitBuffer
		val      int
		length   int
		wantErr  bool
		wantData BitBuffer
	}{
		{
			name:     "test with positive value",
			original: BitBuffer{},
			val:      5,
			length:   3,
			wantErr:  false,
			wantData: BitBuffer{true, false, true},
		},
		{
			name:     "test with negative value",
			original: BitBuffer{},
			val:      -100,
			length:   5,
			wantErr:  true,
			wantData: BitBuffer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.original.appendBits(tt.val, tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("appendBits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.wantData, tt.original)
		})
	}
}

func TestAppendData(t *testing.T) {
	cases := []struct {
		ABufferSet *BitBuffer
		BBufferSet *BitBuffer
		wantErr    bool
		wantData   *BitBuffer
	}{
		{
			ABufferSet: &BitBuffer{},
			BBufferSet: nil,
			wantErr:    true,
			wantData:   &BitBuffer{},
		},
		{
			ABufferSet: &BitBuffer{},
			BBufferSet: &BitBuffer{true, true, false, true, false, true},
			wantErr:    false,
			wantData:   &BitBuffer{true, true, false, true, false, true},
		},
		{
			ABufferSet: &BitBuffer{true, true, false, true, false, false},
			BBufferSet: &BitBuffer{},
			wantErr:    false,
			wantData:   &BitBuffer{true, true, false, true, false, false},
		},
		{
			ABufferSet: &BitBuffer{false, false, true, true, false, true, false, true, true, false, true},
			BBufferSet: &BitBuffer{true, true, false, true, false, false},
			wantErr:    false,
			wantData:   &BitBuffer{false, false, true, true, false, true, false, true, true, false, true, true, true, false, true, false, false},
		},
	}

	for _, tt := range cases {
		err := tt.ABufferSet.appendData(tt.BBufferSet)
		if (err != nil) != tt.wantErr {
			t.Errorf("appendData() error = %v, wantErr %v", err, tt.wantErr)
			return
		}
		assert.Equal(t, tt.wantData, tt.ABufferSet)
	}
}
