package go_qr

import "fmt"

// penaltyN1 - N4 are constants used in QR Code masking penalty rules.
const (
	penaltyN1 = 3
	penaltyN2 = 3
	penaltyN3 = 40
	penaltyN4 = 10
)

// maskInvert reports whether the mask pattern msk inverts the module at (x, y).
func maskInvert(msk, x, y int) bool {
	switch msk {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	case 7:
		return ((x+y)%2+x*y%3)%2 == 0
	}
	return false
}

// applyMask applies the chosen mask pattern to the QR code in place.
func (q *builder) applyMask(msk int) error {
	if msk < 0 || msk > 7 {
		return fmt.Errorf("%w: mask %d out of range [0,7]", ErrInvalidArgument, msk)
	}

	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			invert := maskInvert(msk, x, y)
			q.modules[y][x] = q.modules[y][x] != (invert && !q.isFunction[y][x])
		}
	}
	return nil
}

// writeMaskedGrid writes the modules XORed with a precomputed mask pattern into
// dst without mutating the QR matrix. Used during mask selection so penalty
// scoring can read masked values directly, avoiding the apply/undo passes the
// in-place approach needs. pattern[y][x] is maskInvert(mask, x, y) && !isFunction
// (from the version's cached qrTemplate), so this is a plain XOR with no
// per-module mask formula evaluation.
func (q *builder) writeMaskedGrid(dst, pattern [][]bool) {
	for y := 0; y < q.size; y++ {
		row := q.modules[y]
		pat := pattern[y]
		out := dst[y]
		for x := 0; x < q.size; x++ {
			out[x] = row[x] != pat[x]
		}
	}
}

// getPenaltyScore calculates and returns a penalty score based on several criteria.
//
// cap is an early-abort threshold: once the partial score reaches cap it can no
// longer win the mask comparison (scores only grow), so we bail out and return a
// value >= cap. Pass math.MaxInt32 to force a full, exact computation.
func (q *builder) getPenaltyScore(grid [][]bool, cap int) int {
	res := 0
	size := q.size
	var runHistoryArr [7]int
	runHistory := runHistoryArr[:]
	dark := 0

	// Horizontal pass: a single sweep over the grid that fuses rule 1
	// (run lengths), rule 3 (finder-like patterns), rule 2 (2x2 blocks, scored
	// against the next row), and rule 4's dark-module tally — three of the four
	// rules that previously took their own full O(size^2) passes.
	for y := 0; y < size; y++ {
		row := grid[y]
		var next []bool
		if y+1 < size {
			next = grid[y+1]
		}
		runColor, runX := false, 0
		runHistoryArr = [7]int{}
		for x := 0; x < size; x++ {
			cell := row[x]
			if cell {
				dark++
			}
			if cell == runColor {
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
				runColor = cell
				runX = 1
			}
			// rule 2: 2x2 same-color block with top-left at (x, y).
			if next != nil && x+1 < size &&
				cell == row[x+1] && cell == next[x] && cell == next[x+1] {
				res += penaltyN2
			}
		}
		res += q.finderPenaltyTerminateAndCount(runColor, runX, runHistory) * penaltyN3
	}
	if res >= cap {
		return res
	}

	// Vertical pass: rule 1 + rule 3 in the column direction.
	for x := 0; x < size; x++ {
		runColor, runY := false, 0
		runHistoryArr = [7]int{}
		for y := 0; y < size; y++ {
			cell := grid[y][x]
			if cell == runColor {
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
				runColor = cell
				runY = 1
			}
		}
		res += q.finderPenaltyTerminateAndCount(runColor, runY, runHistory) * penaltyN3
	}
	if res >= cap {
		return res
	}

	// rule 4: dark-module balance, using the tally collected in the first pass.
	total := size * size
	k := (abs(dark*20-total*10)+total-1)/total - 1
	res += k * penaltyN4
	return res
}

// finderPenaltyCountPatterns checks if patterns in runHistory follow the 1:1:3:1:1 finder ratio.
func (q *builder) finderPenaltyCountPatterns(runHistory []int) int {
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

// finderPenaltyTerminateAndCount finalizes the run history and returns the residual finder penalty.
func (q *builder) finderPenaltyTerminateAndCount(currentRunColor bool, currentRunLen int, runHistory []int) int {
	if currentRunColor {
		q.finderPenaltyAddHistory(currentRunLen, runHistory)
		currentRunLen = 0
	}
	currentRunLen += q.size
	q.finderPenaltyAddHistory(currentRunLen, runHistory)
	return q.finderPenaltyCountPatterns(runHistory)
}

// finderPenaltyAddHistory shifts the run-length history and prepends the current run length.
func (q *builder) finderPenaltyAddHistory(currentRunLen int, runHistory []int) {
	if runHistory[0] == 0 {
		currentRunLen += q.size
	}
	// Manual shift instead of copy(): runHistory is always 7 elements, and the
	// builtin copy lowered to a runtime.memmove call that showed up hot in
	// profiles. Indexing [6] first lets the compiler drop the later bounds checks.
	runHistory[6] = runHistory[5]
	runHistory[5] = runHistory[4]
	runHistory[4] = runHistory[3]
	runHistory[3] = runHistory[2]
	runHistory[2] = runHistory[1]
	runHistory[1] = runHistory[0]
	runHistory[0] = currentRunLen
}
