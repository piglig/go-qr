package go_qr

import "fmt"

type Mode struct {
	modeBits         int
	numBitsCharCount []int
}

func newMode(mode int, ccbits ...int) Mode {
	return Mode{
		modeBits:         mode,
		numBitsCharCount: ccbits,
	}
}

func (m Mode) numCharCountBits(ver int) int {
	return m.numBitsCharCount[(ver+7)/17]
}

var (
	Numric       = newMode(0x1, 10, 12, 14)
	Alphanumeric = newMode(0x2, 9, 11, 13)
	Byte         = newMode(0x4, 8, 16, 16)
	Kanji        = newMode(0x8, 8, 10, 12)
	Eci          = newMode(0x7, 0, 0, 0)
)

func (m Mode) getModeBits() int {
	return m.modeBits
}

type QrSegment struct {
}

func MakeBytes(data []byte) (*QrSegment, error) {
	if data == nil {
		return nil, fmt.Errorf("")
	}

	bb := BitBuffer{}
	_ = bb
	return nil, nil
}
