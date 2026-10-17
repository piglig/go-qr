package symbol

import (
	"math"
	"sort"
	"sync"

	"github.com/piglig/go-qr/v2/internal/reedsolomon"
)

// gmdMargin is the redundancy, in bytes per block, that erasure decoding
// leaves unused beyond the misdecode protection codewords. Using all of it,
// erasing every check byte would always yield some codeword.
const gmdMargin = 2

// Pilot is a module known before reading the symbol, and its value.
type Pilot struct {
	X, Y uint8
	Dark bool
}

// pilotCache maps version -> []Pilot. It is kept apart from the
// encoder's templates, which are far larger, so that the grids of any
// version that decoding tries leave little memory behind.
var pilotCache sync.Map

// Pilots returns, row by row, the modules of version known before
// reading the symbol: the function modules other than the format
// information, which depends on the level and mask.
func Pilots(version int) []Pilot {
	if v, ok := pilotCache.Load(version); ok {
		return v.([]Pilot)
	}
	size := version*4 + 17
	b := NewBuilder(version, Low)
	b.DrawFunctionPatterns()
	var pilots []Pilot
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if b.IsFunction[y][x] && !IsFormatModule(x, y, size) {
				pilots = append(pilots, Pilot{uint8(x), uint8(y), b.Modules[y][x]})
			}
		}
	}
	actual, _ := pilotCache.LoadOrStore(version, pilots)
	return actual.([]Pilot)
}

// IsFormatModule reports whether module (x, y) of a symbol n modules wide
// holds format information, or the dark module beside it.
func IsFormatModule(x, y, n int) bool {
	return y == 8 && (x <= 8 || x >= n-8) || x == 8 && (y <= 8 || y >= n-8)
}

// codewordReliability returns the reliability of each raw codeword of q's
// version, in placement order, from that of each module, row by row: that
// of its least reliable module. It walks the placement as readCodewords
// does.
func (q *Builder) CodewordReliability(rel []float32) []float32 {
	n := NumRawDataModules(q.Version) / 8
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(math.Inf(1))
	}
	i := 0
	for right := q.Size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.Size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if ((right + 1) & 2) == 0 {
					y = q.Size - 1 - vert
				}
				if !q.IsFunction[y][x] && i < n*8 {
					out[i>>3] = min(out[i>>3], rel[y*q.Size+x])
					i++
				}
			}
		}
	}
	return out
}

// softCorrect corrects a block of an attempt beyond the first hard decision:
// by hard decision within budget, then by erasing its 2, 4, ... least
// reliable bytes, rel[i] being byte i's reliability. It returns the
// positions it erased.
func softCorrect(codeword []byte, eccLen int, rel []float32, budget int) ([]int, error) {
	err := reedsolomon.CorrectErasures(codeword, eccLen, nil, budget)
	if err == nil || budget < 2 {
		return nil, err
	}
	order := make([]int, len(codeword))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return rel[order[a]] < rel[order[b]] })
	try := make([]byte, len(codeword))
	for s := 2; s <= budget; s += 2 {
		copy(try, codeword)
		if reedsolomon.CorrectErasures(try, eccLen, order[:s], budget) == nil {
			copy(codeword, try)
			return order[:s], nil
		}
	}
	return nil, err
}
