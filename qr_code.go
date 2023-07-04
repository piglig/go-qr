package go_qr

import (
	"errors"
	"fmt"
)

type Ecc int

const (
	Low      Ecc = 1
	Medium   Ecc = 0
	Quartile Ecc = 3
	High     Ecc = 2
)

const (
	MinVersion = 1
	MaxVersion = 40
)

const (
	penaltyN1 = 3
	penaltyN2 = 3
	penaltyN3 = 40
	penaltyN4 = 10
)

func getEccCodeWordsPerBlock() [][]int8 {
	return [][]int8{
		// Version: (note that index 0 is for padding, and is set to an illegal value)
		//0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
		{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},  // Low
		{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28}, // Medium
		{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // Quartile
		{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30}, // High
	}
}

func getNumErrorCorrectionBlocks() [][]int8 {
	return [][]int8{
		// Version: (note that index 0 is for padding, and is set to an illegal value)
		//0, 1, 2, 3, 4, 5, 6, 7, 8, 9,10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40    Error correction level
		{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},              // Low
		{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},     // Medium
		{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},  // Quartile
		{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81}, // High
	}
}

type QrCode struct {
	version              int
	size                 int
	errorCorrectionLevel Ecc
	mask                 int

	modules    [][]bool
	isFunction [][]bool
}

func NewQrCode(ver int, ecl Ecc, dataCodewords []byte, msk int) (*QrCode, error) {
	if ver < MinVersion || ver > MaxVersion {
		return nil, errors.New("version value out of range")
	}

	if msk < -1 || msk > 7 {
		return nil, errors.New("mask value out of range")
	}

	if dataCodewords == nil {
		return nil, errors.New("dataCodewords is nil")
	}
	// TODO create QrCode
	return nil, nil
}

func (q *QrCode) setFunctionModule(x, y int, isDark bool) {
	q.modules[y][x] = isDark
	q.isFunction[y][x] = true
}

func (q *QrCode) drawAlignmentPattern(x, y int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			q.setFunctionModule(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

func (q *QrCode) drawFinderPattern(x, y int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := max(abs(dx), abs(dy))
			xx, yy := x+dx, y+dy
			if 0 <= xx && xx < q.size && 0 <= yy && y < q.size {
				q.setFunctionModule(xx, yy, dist != 2 && dist != 4)
			}
		}
	}
}

func (q *QrCode) drawVersion() {
	// TODO drawVersion
}

func EncodeText(text string, ecl Ecc) (*QrCode, error) {
	segs, err := MakeSegments(text)
	if err != nil {
		return nil, err
	}

	_ = segs
	return nil, nil
}

func encodeSegments(segs []*QrSegment, ecl Ecc, minVer, maxVer, mask int, boostEcl bool) (*QrCode, error) {
	if !(MinVersion <= minVer && minVer <= maxVer && maxVer <= MaxVersion) {
		return nil, errors.New("invalid value")
	}

	version, dataUsedBits := 0, 0
	for version = minVer; ; version++ {
		dataCapacityBits, err := getNumDataCodewords(version, ecl)
		if err != nil {
			return nil, err
		}
		dataCapacityBits *= 8
		dataUsedBits = getTotalBits(segs, version)
		if dataUsedBits != -1 && dataUsedBits <= dataCapacityBits {
			break
		}

		if version >= maxVer {
			msg := "Segment too long"
			if dataUsedBits != -1 {
				msg = fmt.Sprintf("Data length = %d bits, Max capacity = %d bits", dataUsedBits, dataCapacityBits)
			}
			return nil, &DataTooLongException{Msg: msg}
		}
	}

	for _, newEcl := range []Ecc{Medium, Quartile, High} {
		numDataCodewords, err := getNumDataCodewords(version, newEcl)
		if err != nil {
			return nil, err
		}
		if boostEcl && dataUsedBits <= numDataCodewords*8 {
			ecl = newEcl
		}
	}

	bb := BitBuffer{}
	for _, seg := range segs {
		if seg == nil {
			continue
		}

		err := bb.appendBits(seg.mode.modeBits, 4)
		if err != nil {
			return nil, err
		}
		err = bb.appendBits(seg.numChars, seg.mode.numCharCountBits(version))
		if err != nil {
			return nil, err
		}
		err = bb.appendData(seg.data)
		if err != nil {
			return nil, err
		}
	}

	dataCapacityBits, err := getNumDataCodewords(version, ecl)
	if err != nil {
		return nil, err
	}

	dataCapacityBits *= 8
	err = bb.appendBits(0, min(4, dataCapacityBits-bb.len()))
	if err != nil {
		return nil, err
	}

	err = bb.appendBits(0, (8-bb.len()%8)%8)
	if err != nil {
		return nil, err
	}

	for padByte := 0xEC; bb.len() < dataCapacityBits; padByte ^= 0xEC ^ 0x11 {
		err = bb.appendBits(padByte, 8)
		if err != nil {
			return nil, err
		}
	}

	dataCodewords := make([]byte, bb.len()/8)
	for i := 0; i < bb.len(); i++ {
		bit := 0
		if bb.getBit(i) {
			bit = 1
		}
		dataCodewords[i>>3] |= byte(bit << (7 - (i & 7)))
	}
	return &QrCode{}, nil
}

func getNumDataCodewords(ver int, ecl Ecc) (int, error) {
	eccCodewordsPerBlock := getEccCodeWordsPerBlock()
	numRawDataModules, err := getNumRawDataModules(ver)
	if err != nil {
		return 0, err
	}

	numErrorCorrectionBlocks := getNumErrorCorrectionBlocks()
	return numRawDataModules/8 -
		int(eccCodewordsPerBlock[ecl][ver])*int(numErrorCorrectionBlocks[ecl][ver]), nil
}

func getNumRawDataModules(ver int) (int, error) {
	if ver < MinVersion || ver > MaxVersion {
		return 0, errors.New("version number out of range")
	}

	size := ver*4 + 17
	res := size * size
	res -= 8 * 8 * 3
	res -= 15*2 + 1
	res -= (size - 16) * 2

	if ver >= 2 {
		numAlign := ver/7 + 2
		res -= (numAlign - 1) * (numAlign - 1) * 25
		res -= (numAlign - 2) * 2 * 20

		if ver >= 7 {
			res -= 6 * 3 * 2
		}
	}
	return res, nil
}

func GetBit(x, i int) bool {
	return ((x >> uint(i)) & 1) != 0
}
