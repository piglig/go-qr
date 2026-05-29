package go_qr

// abs returns the absolute value of x. (min and max use the language builtins.)
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
