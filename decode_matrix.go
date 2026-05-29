package go_qr

import (
	"fmt"

	"github.com/piglig/go-qr/internal/reedsolomon"
)

// decodeMatrix takes a fully sampled module grid (modules[y][x] == dark) and
// recovers the data codewords: read format info, undo the mask, walk the
// codeword zig-zag, de-interleave the ECC blocks, and Reed-Solomon correct.
//
// It mirrors the encoder geometry exactly by reusing drawFunctionPatterns to
// rebuild the function-module map and applyMask (which is its own inverse) to
// remove the mask.
func decodeMatrix(modules [][]bool) (data []byte, ver int, ecl Ecc, mask int, err error) {
	size := len(modules)
	if size < 21 || (size-17)%4 != 0 {
		return nil, 0, 0, 0, fmt.Errorf("%w: bad matrix size %d", ErrDecodeFailed, size)
	}
	ver = (size - 17) / 4
	if ver < MinVersion || ver > MaxVersion {
		return nil, 0, 0, 0, fmt.Errorf("%w: derived version %d", ErrDecodeFailed, ver)
	}

	// Read and error-correct the format bits before touching the data region.
	ecl, mask, err = readFormat(modules, size)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	// Reuse a builder purely to rebuild the version's function-pattern map, then
	// overlay the sampled modules and undo the mask.
	b := newBuilder(ver, ecl)
	b.drawFunctionPatterns() // populates isFunction (modules get overwritten next)
	for y := 0; y < size; y++ {
		copy(b.modules[y], modules[y])
	}
	if err = b.applyMask(mask); err != nil { // XOR is self-inverse → unmask
		return nil, 0, 0, 0, err
	}

	raw := b.readCodewords()
	data, err = deinterleaveAndCorrect(raw, ver, ecl)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return data, ver, ecl, mask, nil
}

// readFormat reads the 15-bit format information (two redundant copies),
// strips the 0x5412 mask, BCH-corrects it, and returns the ECC level and mask.
func readFormat(modules [][]bool, size int) (Ecc, int, error) {
	bitAt := func(x, y int) int {
		if modules[y][x] {
			return 1
		}
		return 0
	}

	// Primary copy around the top-left finder (reverse of drawFormatBits).
	var primary int
	for i := 0; i <= 5; i++ {
		primary |= bitAt(8, i) << i
	}
	primary |= bitAt(8, 7) << 6
	primary |= bitAt(8, 8) << 7
	primary |= bitAt(7, 8) << 8
	for i := 9; i < 15; i++ {
		primary |= bitAt(14-i, 8) << i
	}

	// Secondary copy along the right column / bottom row.
	var secondary int
	for i := 0; i < 8; i++ {
		secondary |= bitAt(size-1-i, 8) << i
	}
	for i := 8; i < 15; i++ {
		secondary |= bitAt(8, size-15+i) << i
	}

	data, ok := correctFormat(primary)
	if !ok {
		data, ok = correctFormat(secondary)
	}
	if !ok {
		return 0, 0, fmt.Errorf("%w: unreadable format info", ErrDecodeFailed)
	}

	formatVal := data >> 3 // 2 bits
	mask := data & 0x7     // 3 bits
	// eccFormats {Low:1, Medium:0, Quartile:3, High:2} is an involution, so the
	// same table maps the 2-bit format value back to an Ecc.
	ecl := Ecc(eccFormats[formatVal])
	return ecl, mask, nil
}

// correctFormat removes the 0x5412 mask and finds the nearest of the 32 valid
// BCH(15,5) codewords by Hamming distance. Returns the 5-bit data and whether a
// unique correction within 3 bit errors was found.
func correctFormat(raw int) (int, bool) {
	unmasked := raw ^ 0x5412
	bestData, bestDist := -1, 99
	for d := 0; d < 32; d++ {
		code := formatBCH(d)
		dist := bitCount(code ^ unmasked)
		if dist < bestDist {
			bestDist, bestData = dist, d
		}
	}
	if bestDist <= 3 {
		return bestData, true
	}
	return 0, false
}

// formatBCH builds the 15-bit BCH codeword for a 5-bit format data value,
// matching the encoder's polynomial (mask not applied here).
func formatBCH(data int) int {
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	return data<<10 | rem
}

func bitCount(x int) int {
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}

// readCodewords reverses drawCodewords: walk columns right-to-left in pairs,
// zig-zagging, reading 8 bits per codeword from non-function modules.
func (q *builder) readCodewords() []byte {
	n := getNumRawDataModules(q.version) / 8
	data := make([]byte, n)
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
				if !q.isFunction[y][x] && i < n*8 {
					if q.modules[y][x] {
						data[i>>3] |= 1 << uint(7-(i&7))
					}
					i++
				}
			}
		}
	}
	return data
}

// deinterleaveAndCorrect splits the interleaved raw codewords back into ECC
// blocks (reverse of addEccAndInterLeave), Reed-Solomon corrects each block,
// and concatenates the corrected data codewords in order.
func deinterleaveAndCorrect(raw []byte, ver int, ecl Ecc) ([]byte, error) {
	numBlocks := int(numErrorCorrectionBlocks[ecl][ver])
	blockEccLen := int(eccCodeWordsPerBlock[ecl][ver])
	rawCodewords := getNumRawDataModules(ver) / 8

	numShortBlocks := numBlocks - rawCodewords%numBlocks
	shortBlockLen := rawCodewords / numBlocks
	shortDataLen := shortBlockLen - blockEccLen // data bytes in a short block

	// Mirror the encoder's layout exactly: every block buffer is shortBlockLen+1
	// long, and short blocks leave an unused "gap" byte at index shortDataLen.
	// The interleave skips that gap; the inverse must too.
	blocks := make([][]byte, numBlocks)
	for i := range blocks {
		blocks[i] = make([]byte, shortBlockLen+1)
	}
	k := 0
	for i := 0; i <= shortBlockLen; i++ {
		for j := 0; j < numBlocks; j++ {
			if i != shortDataLen || j >= numShortBlocks {
				blocks[j][i] = raw[k]
				k++
			}
		}
	}

	// Build the contiguous RS codeword (data | ecc) per block, dropping the gap
	// in short blocks, correct it, and collect the corrected data.
	var out []byte
	for j, block := range blocks {
		dataLen := shortDataLen
		if j >= numShortBlocks {
			dataLen = shortDataLen + 1 // long block has one extra data byte
		}
		codeword := make([]byte, 0, dataLen+blockEccLen)
		codeword = append(codeword, block[:dataLen]...)                     // data (gap excluded)
		codeword = append(codeword, block[shortBlockLen+1-blockEccLen:]...) // ecc
		if err := reedsolomon.Correct(codeword, blockEccLen); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
		}
		out = append(out, codeword[:dataLen]...)
	}
	return out, nil
}
