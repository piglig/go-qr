package go_qr

import (
	"errors"
	"fmt"
	"math"
)

type Ecc int

const (
	Low Ecc = iota
	Medium
	Quartile
	High
)

var eccFormats = [...]int{1, 0, 3, 2}

func (e Ecc) FormatBits() int {
	return eccFormats[e]
}

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

	qrCode := &QrCode{
		version:              ver,
		size:                 ver*4 + 17,
		errorCorrectionLevel: ecl,
	}

	modules := make([][]bool, qrCode.size)
	isFunction := make([][]bool, qrCode.size)
	for i := 0; i < qrCode.size; i++ {
		modules[i] = make([]bool, qrCode.size)
		isFunction[i] = make([]bool, qrCode.size)
	}
	qrCode.modules = modules
	qrCode.isFunction = isFunction

	qrCode.drawFunctionPatterns()
	allCodewords, err := qrCode.addEccAndInterLeave(dataCodewords)
	if err != nil {
		return nil, err
	}

	err = qrCode.drawCodewords(allCodewords)
	if err != nil {
		return nil, err
	}

	if msk == -1 {
		minPenalty := math.MaxInt32
		for i := 0; i < 8; i++ {
			err = qrCode.applyMask(i)
			if err != nil {
				return nil, err
			}
			qrCode.drawFormatBits(i)
			penalty := qrCode.getPenaltyScore()
			if penalty < minPenalty {
				msk = i
				minPenalty = penalty
			}
			err = qrCode.applyMask(i)
			if err != nil {
				return nil, err
			}
		}
	}

	qrCode.mask = msk
	err = qrCode.applyMask(msk)
	if err != nil {
		return nil, err
	}

	qrCode.drawFormatBits(msk)
	qrCode.isFunction = nil

	return qrCode, nil
}

func (q *QrCode) GetSize() int {
	return q.size
}

func (q *QrCode) GetModule(x, y int) bool {
	return 0 <= x && x < q.size && 0 <= y && y < q.size && q.modules[y][x]
}

func (q *QrCode) setFunctionModule(x, y int, isDark bool) {
	q.modules[y][x] = isDark
	q.isFunction[y][x] = true
}

func (q *QrCode) addEccAndInterLeave(data []byte) ([]byte, error) {
	if data == nil {
		return nil, errors.New("data is nil")
	}

	numDataCodewords := getNumDataCodewords(q.version, q.errorCorrectionLevel)

	if len(data) != numDataCodewords {
		return nil, errors.New("invalid argument")
	}

	numBlocks := getNumErrorCorrectionBlocks()[q.errorCorrectionLevel][q.version]
	blockEccLen := getEccCodeWordsPerBlock()[q.errorCorrectionLevel][q.version]
	rawCodewords := getNumRawDataModules(q.version) / 8
	numShortBlocks := int(numBlocks) - rawCodewords%int(numBlocks)
	shortBlockLen := rawCodewords / int(numBlocks)

	blocks := make([][]byte, numBlocks)
	rsDiv, err := reedSolomonComputeDivisor(int(blockEccLen))
	if err != nil {
		return nil, err
	}
	for i, k := 0, 0; i < int(numBlocks); i++ {
		index := 1
		if i < numShortBlocks {
			index = 0
		}
		dat := make([]byte, shortBlockLen-int(blockEccLen)+index)
		copy(dat, data[k:k+shortBlockLen-int(blockEccLen)+index])
		k += len(dat)
		block := make([]byte, shortBlockLen+1)
		copy(block, dat)
		ecc := reedSolomonComputeRemainder(dat, rsDiv)
		copy(block[len(block)-int(blockEccLen):], ecc)
		blocks[i] = block
	}

	res := make([]byte, rawCodewords)
	for i, k := 0, 0; i < len(blocks[0]); i++ {
		for j := 0; j < len(blocks); j++ {
			if i != shortBlockLen-int(blockEccLen) || j >= numShortBlocks {
				res[k] = blocks[j][i]
				k++
			}
		}
	}
	return res, nil
}

func (q *QrCode) drawCodewords(data []byte) error {
	numRawDataModules := getNumRawDataModules(q.version) / 8
	if len(data) != numRawDataModules {
		return errors.New("illegal argument")
	}

	i := 0
	for right := q.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := ((right + 1) & 2) == 0
				y := vert
				if upward {
					y = q.size - 1 - vert
				}
				if !q.isFunction[y][x] && i < len(data)*8 {
					q.modules[y][x] = getBit(int(data[i>>3]), 7-(i&7))
					i++
				}
			}
		}
	}
	return nil
}

func (q *QrCode) applyMask(msk int) error {
	if msk < 0 || msk > 7 {
		return errors.New("mask value out of range")
	}

	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			var invert bool
			switch msk {
			case 0:
				invert = (x+y)%2 == 0
			case 1:
				invert = y%2 == 0
			case 2:
				invert = x%3 == 0
			case 3:
				invert = (x+y)%3 == 0
			case 4:
				invert = (x/3+y/2)%2 == 0
			case 5:
				invert = x*y%2+x*y%3 == 0
			case 6:
				invert = (x*y%2+x*y%3)%2 == 0
			case 7:
				invert = ((x+y)%2+x*y%3)%2 == 0
			default:
				return errors.New("mask value out of range")
			}
			q.modules[y][x] = q.modules[y][x] != (invert && !q.isFunction[y][x])
		}
	}
	return nil
}

func (q *QrCode) getPenaltyScore() int {
	res := 0
	for y := 0; y < q.size; y++ {
		runColor, runX := false, 0
		runHistory := make([]int, 7)
		for x := 0; x < q.size; x++ {
			if q.modules[y][x] == runColor {
				runX++
				if runX == 5 {
					res += penaltyN1
				} else if runX > 5 {
					res++
				}
			} else {
				q.finderPenaltyAddHistory(runX, runHistory)
				if !runColor {
					res += q.finderPenaltyCountPatterns(runHistory) * penaltyN3
				}
				runColor = q.modules[y][x]
				runX = 1
			}
		}
		res += q.finderPenaltyTerminateAndCount(runColor, runX, runHistory) * penaltyN3
	}

	for x := 0; x < q.size; x++ {
		runColor, runY := false, 0
		runHistory := make([]int, 7)
		for y := 0; y < q.size; y++ {
			if q.modules[y][x] == runColor {
				runY++
				if runY == 5 {
					res += penaltyN1
				} else if runY > 5 {
					res++
				}
			} else {
				q.finderPenaltyAddHistory(runY, runHistory)
				if !runColor {
					res += q.finderPenaltyCountPatterns(runHistory) * penaltyN3
				}
				runColor = q.modules[y][x]
				runY = 1
			}
		}
		res += q.finderPenaltyTerminateAndCount(runColor, runY, runHistory) * penaltyN3
	}

	for y := 0; y < q.size-1; y++ {
		for x := 0; x < q.size-1; x++ {
			color := q.modules[y][x]
			if color == q.modules[y][x+1] &&
				color == q.modules[y+1][x] &&
				color == q.modules[y+1][x+1] {
				res += penaltyN2
			}
		}
	}

	dark := 0
	for _, row := range q.modules {
		for _, color := range row {
			if color {
				dark++
			}
		}
	}

	total := q.size * q.size
	k := (abs(dark*20-total*10)+total-1)/total - 1
	res += k * penaltyN4
	return res
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
			if 0 <= xx && xx < q.size && 0 <= yy && yy < q.size {
				q.setFunctionModule(xx, yy, dist != 2 && dist != 4)
			}
		}
	}
}

func (q *QrCode) drawVersion() {
	if q.version < 7 {
		return
	}

	rem := q.version
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	bits := q.version<<12 | rem

	// Draw two copies
	for i := 0; i < 18; i++ {
		bit := getBit(bits, i)
		a := q.size - 11 + i%3
		b := i / 3
		q.setFunctionModule(a, b, bit)
		q.setFunctionModule(b, a, bit)
	}
}

func (q *QrCode) drawFunctionPatterns() {
	for i := 0; i < q.size; i++ {
		q.setFunctionModule(6, i, i%2 == 0)
		q.setFunctionModule(i, 6, i%2 == 0)
	}

	q.drawFinderPattern(3, 3)
	q.drawFinderPattern(q.size-4, 3)
	q.drawFinderPattern(3, q.size-4)

	alignPatPos := q.getAlignmentPatternPositions()
	numAlign := len(alignPatPos)
	for i := 0; i < numAlign; i++ {
		for j := 0; j < numAlign; j++ {
			if !(i == 0 && j == 0 || i == 0 && j == numAlign-1 || i == numAlign-1 && j == 0) {
				q.drawAlignmentPattern(alignPatPos[i], alignPatPos[j])
			}
		}
	}

	q.drawFormatBits(0)
	q.drawVersion()
}

func (q *QrCode) getAlignmentPatternPositions() []int {
	if q.version == 1 {
		return []int{}
	} else {
		numAlign := q.version/7 + 2
		step := 0
		if q.version == 32 {
			step = 26
		} else {
			step = (q.version*4 + numAlign*2 + 1) / (numAlign*2 - 2) * 2
		}

		res := make([]int, numAlign)
		res[0] = 6
		for i, pos := len(res)-1, q.size-7; i >= 1; {
			res[i] = pos
			i--
			pos -= step
		}

		return res
	}
}

func (q *QrCode) drawFormatBits(msk int) {
	data := q.errorCorrectionLevel.FormatBits()<<3 | msk
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}

	bits := (data<<10 | rem) ^ 0x5412

	for i := 0; i <= 5; i++ {
		q.setFunctionModule(8, i, getBit(bits, i))
	}
	q.setFunctionModule(8, 7, getBit(bits, 6))
	q.setFunctionModule(8, 8, getBit(bits, 7))
	q.setFunctionModule(7, 8, getBit(bits, 8))

	for i := 9; i < 15; i++ {
		q.setFunctionModule(14-i, 8, getBit(bits, i))
	}

	for i := 0; i < 8; i++ {
		q.setFunctionModule(q.size-1-i, 8, getBit(bits, i))
	}

	for i := 8; i < 15; i++ {
		q.setFunctionModule(8, q.size-15+i, getBit(bits, i))
	}
	q.setFunctionModule(8, q.size-8, true)
}

func EncodeText(text string, ecl Ecc) (*QrCode, error) {
	segs, err := MakeSegments(text)
	if err != nil {
		return nil, err
	}

	return EncodeStandardSegments(segs, ecl)
}

func EncodeBinary(data []byte, ecl Ecc) (*QrCode, error) {
	segs, err := MakeBytes(data)
	if err != nil {
		return nil, err
	}

	return EncodeStandardSegments([]*QrSegment{segs}, ecl)
}

func EncodeStandardSegments(segs []*QrSegment, ecl Ecc) (*QrCode, error) {
	return EncodeSegments(segs, ecl, MinVersion, MaxVersion, -1, true)
}

func EncodeSegments(segs []*QrSegment, ecl Ecc, minVer, maxVer, mask int, boostEcl bool) (*QrCode, error) {
	if !isValidVersion(minVer, maxVer) {
		return nil, errors.New("invalid value")
	}

	version, dataUsedBits := 0, 0
	for version = minVer; ; version++ {
		dataCapacityBits := getNumDataCodewords(version, ecl) * 8
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
		numDataCodewords := getNumDataCodewords(version, newEcl)
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

	dataCapacityBits := getNumDataCodewords(version, ecl) * 8
	err := bb.appendBits(0, min(4, dataCapacityBits-bb.len()))
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
	return NewQrCode(version, ecl, dataCodewords, mask)
}

func isValidVersion(minVer, maxVer int) bool {
	return MinVersion <= minVer && minVer <= maxVer && maxVer <= MaxVersion
}

func getNumDataCodewords(ver int, ecl Ecc) int {
	eccCodewordsPerBlock := getEccCodeWordsPerBlock()
	numRawDataModules := getNumRawDataModules(ver)
	numErrorCorrectionBlocks := getNumErrorCorrectionBlocks()
	return numRawDataModules/8 -
		int(eccCodewordsPerBlock[ecl][ver])*int(numErrorCorrectionBlocks[ecl][ver])
}

func (q *QrCode) finderPenaltyCountPatterns(runHistory []int) int {
	n := runHistory[1]
	core := n > 0 && runHistory[2] == n && runHistory[3] == n*3 && runHistory[4] == n && runHistory[5] == n
	res := 0
	if core && runHistory[0] >= n*4 && runHistory[6] >= n {
		res = 1
	}

	if core && runHistory[6] >= n*4 && runHistory[0] >= n {
		res += 1
	}
	return res
}

func (q *QrCode) finderPenaltyTerminateAndCount(currentRunColor bool, currentRunLen int, runHistory []int) int {
	if currentRunColor {
		q.finderPenaltyAddHistory(currentRunLen, runHistory)
		currentRunLen = 0
	}

	currentRunLen += q.size
	q.finderPenaltyAddHistory(currentRunLen, runHistory)
	return q.finderPenaltyCountPatterns(runHistory)
}

func getNumRawDataModules(ver int) int {
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
	return res
}

func reedSolomonComputeDivisor(degree int) ([]byte, error) {
	if degree < 1 || degree > 255 {
		return nil, errors.New("degree out of range")
	}

	res := make([]byte, degree)
	res[degree-1] = 1

	root := 1
	for i := 0; i < degree; i++ {
		for j := 0; j < len(res); j++ {
			res[j] = byte(reedSolomonMultiply(int(res[j]&0xFF), root))
			if j+1 < len(res) {
				res[j] ^= res[j+1]
			}
		}
		root = reedSolomonMultiply(root, 0x02)
	}
	return res, nil
}

func reedSolomonComputeRemainder(data, divisor []byte) []byte {
	res := make([]byte, len(divisor))
	for _, b := range data {
		factor := (b ^ res[0]) & 0xFF
		copy(res, res[1:])
		res[len(res)-1] = byte(0)
		for i := 0; i < len(res); i++ {
			res[i] ^= byte(reedSolomonMultiply(int(divisor[i]&0xFF), int(factor)))
		}
	}
	return res
}

func reedSolomonMultiply(x, y int) int {
	z := 0
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= ((y >> i) & 1) * x
	}
	return z
}

func (q *QrCode) finderPenaltyAddHistory(currentRunLen int, runHistory []int) {
	if runHistory[0] == 0 {
		currentRunLen += q.size
	}
	copy(runHistory[1:], runHistory[:len(runHistory)-1])
	runHistory[0] = currentRunLen
}

func getBit(x, i int) bool {
	return ((x >> uint(i)) & 1) != 0
}
