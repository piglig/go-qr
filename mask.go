package go_qr

import "fmt"

// penaltyN1 - N4 are constants used in QR Code masking penalty rules.
const (
	penaltyN1 = 3
	penaltyN2 = 3
	penaltyN3 = 40
	penaltyN4 = 10
)

// applyMask applies the chosen mask pattern to the QR code.
func (q *QrCode) applyMask(msk int) error {
	if msk < 0 || msk > 7 {
		return fmt.Errorf("%w: mask %d out of range [0,7]", ErrInvalidArgument, msk)
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
			}
			q.modules[y][x] = q.modules[y][x] != (invert && !q.isFunction[y][x])
		}
	}
	return nil
}

// getPenaltyScore calculates and returns a penalty score based on several criteria.
func (q *QrCode) getPenaltyScore() int {
	res := 0
	// Horizontal runs
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

	// Vertical runs
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

	// 2x2 same-color blocks
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

	// Dark-module balance
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

// finderPenaltyCountPatterns checks if patterns in runHistory follow the 1:1:3:1:1 finder ratio.
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

// finderPenaltyTerminateAndCount finalizes the run history and returns the residual finder penalty.
func (q *QrCode) finderPenaltyTerminateAndCount(currentRunColor bool, currentRunLen int, runHistory []int) int {
	if currentRunColor {
		q.finderPenaltyAddHistory(currentRunLen, runHistory)
		currentRunLen = 0
	}
	currentRunLen += q.size
	q.finderPenaltyAddHistory(currentRunLen, runHistory)
	return q.finderPenaltyCountPatterns(runHistory)
}

// finderPenaltyAddHistory shifts the run-length history and prepends the current run length.
func (q *QrCode) finderPenaltyAddHistory(currentRunLen int, runHistory []int) {
	if runHistory[0] == 0 {
		currentRunLen += q.size
	}
	copy(runHistory[1:], runHistory[:len(runHistory)-1])
	runHistory[0] = currentRunLen
}
