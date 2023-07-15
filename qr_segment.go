package go_qr

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
)

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
	Numeric      = newMode(0x1, 10, 12, 14)
	Alphanumeric = newMode(0x2, 9, 11, 13)
	Byte         = newMode(0x4, 8, 16, 16)
	Kanji        = newMode(0x8, 8, 10, 12)
	Eci          = newMode(0x7, 0, 0, 0)
)

func (m Mode) getModeBits() int {
	return m.modeBits
}

func (m Mode) isNumeric() bool {
	return m.modeBits == Numeric.getModeBits()
}

func (m Mode) isAlphanumeric() bool {
	return m.modeBits == Alphanumeric.getModeBits()
}

func (m Mode) isByte() bool {
	return m.modeBits == Byte.getModeBits()
}

func (m Mode) isKanji() bool {
	return m.modeBits == Kanji.getModeBits()
}

func (m Mode) isEci() bool {
	return m.modeBits == Eci.getModeBits()
}

const (
	NumericRegex        = `\d`
	AlphanumericRegex   = `^[0-9A-Z $%*+\-.\/:]*$`
	AlphanumericCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"
)

type QrSegment struct {
	mode     Mode
	numChars int
	data     *BitBuffer
}

func newQrSegment(mode Mode, numCh int, data *BitBuffer) (*QrSegment, error) {
	if numCh < 0 {
		return nil, errors.New("invalid value")
	}
	return &QrSegment{
		mode:     mode,
		numChars: numCh,
		data:     data.clone(),
	}, nil
}

func (q *QrSegment) getData() *BitBuffer {
	return q.data.clone()
}

func MakeBytes(data []byte) (*QrSegment, error) {
	if data == nil {
		return nil, errors.New("data is nil")
	}

	bb := &BitBuffer{}
	for _, b := range data {
		err := bb.appendBits(int(b&0xFF), 8)
		if err != nil {
			return nil, err
		}
	}
	return newQrSegment(Byte, len(data), bb)
}

func MakeNumeric(digits string) (*QrSegment, error) {
	if !isNumeric(digits) {
		return nil, errors.New("string contains non-numeric characters")
	}

	bb := &BitBuffer{}
	for i := 0; i < len(digits); {
		n := min(len(digits)-i, 3)
		num, _ := strconv.Atoi(digits[i : i+n])
		err := bb.appendBits(num, n*3+1)
		if err != nil {
			return nil, err
		}
		i += n
	}

	return newQrSegment(Numeric, len(digits), bb)
}

func isNumeric(numb string) bool {
	return regexp.MustCompile(NumericRegex).MatchString(numb)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func MakeAlphanumeric(text string) (*QrSegment, error) {
	if !isisAlphanumeric(text) {
		return nil, errors.New("string contains unencodable characters in alphanumeric mode")
	}

	bb := &BitBuffer{}
	i := 0
	for ; i <= len(text)-2; i += 2 {
		temp := strings.IndexByte(AlphanumericCharset, text[i]) * 45
		temp += strings.IndexByte(AlphanumericCharset, text[i+1])
		err := bb.appendBits(temp, 11)
		if err != nil {
			return nil, err
		}
	}

	if i < len(text) {
		err := bb.appendBits(strings.IndexByte(AlphanumericCharset, text[i]), 6)
		if err != nil {
			return nil, err
		}
	}

	return newQrSegment(Alphanumeric, len(text), bb)
}

func isisAlphanumeric(text string) bool {
	return regexp.MustCompile(AlphanumericRegex).MatchString(text)
}

func MakeSegments(text string) ([]*QrSegment, error) {
	res := make([]*QrSegment, 0)
	if text == "" {
	} else if isNumeric(text) {
		seg, err := MakeNumeric(text)
		if err != nil {
			return nil, err
		}

		res = append(res, seg)
	} else if isisAlphanumeric(text) {
		seg, err := MakeAlphanumeric(text)
		if err != nil {
			return nil, err
		}
		res = append(res, seg)
	} else {
		seg, err := MakeBytes([]byte(text))
		if err != nil {
			return nil, err
		}
		res = append(res, seg)
	}
	return res, nil
}

func MakeEci(val int) (*QrSegment, error) {
	bb := &BitBuffer{}
	if val < 0 {
		return nil, errors.New("ECI assignment value out of range")
	} else if val < (1 << 7) {
		err := bb.appendBits(val, 8)
		if err != nil {
			return nil, err
		}
	} else if val < (1 << 14) {
		err := bb.appendBits(0b10, 2)
		if err != nil {
			return nil, err
		}

		err = bb.appendBits(val, 14)
		if err != nil {
			return nil, err
		}
	} else if val < 1e6 {
		err := bb.appendBits(0b110, 3)
		if err != nil {
			return nil, err
		}

		err = bb.appendBits(val, 21)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("ECI assignment value out of range")
	}
	return newQrSegment(Eci, 0, bb)
}

func getTotalBits(segs []*QrSegment, ver int) int {
	var res int64
	for _, seg := range segs {
		if seg == nil {
			continue
		}

		ccbits := seg.mode.numCharCountBits(ver)
		if seg.numChars >= (1 << ccbits) {
			return -1
		}
		res += int64(4 + ccbits + seg.data.len())
		if res > math.MaxInt {
			return -1
		}
	}
	return int(res)
}
