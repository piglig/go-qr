// Package reedsolomon implements Reed-Solomon error correction over GF(2^8)
// with the QR Code field polynomial 0x11D.
//
// It is a self-contained algebra package: the encoder side computes a
// generator polynomial (Divisor) and the codeword remainder (Remainder); the
// decoder side recovers a possibly-corrupted codeword (Correct) via syndrome
// evaluation, the Berlekamp-Massey algorithm, a Chien search, and Forney's
// algorithm, and with some positions known to be unreliable
// (CorrectErasures) through Forney syndromes. The Galois-field tables and the generator-polynomial cache are
// built once, in a single ordered init, with no external dependencies.
package reedsolomon

import (
	"errors"
	"fmt"
)

// ErrUncorrectable indicates a received codeword has more errors than the ECC
// bytes can repair.
var ErrUncorrectable = errors.New("reed-solomon: uncorrectable codeword")

// ErrInvalidDegree is returned by Divisor for a degree outside [1, 255].
var ErrInvalidDegree = errors.New("reed-solomon: generator degree out of range")

var (
	gfExp [512]byte // gfExp[i] = generator^i (doubled length to avoid mod on multiply)
	gfLog [256]byte // gfLog[x] = i such that generator^i = x
)

// maxDivisorDegree is the largest ECC-per-block degree the QR spec uses.
const maxDivisorDegree = 30

// divisorCache holds the precomputed generator polynomials for every degree in
// [1, maxDivisorDegree]. They depend only on the degree, so caching avoids
// recomputing on every encode; the slices are read-only after init and safe to
// share across concurrent encodes.
var divisorCache [maxDivisorDegree + 1][]byte

func init() {
	// 1. Build the log/antilog tables from the generator (0x02).
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x = gfMultiplyBitwise(x, 0x02)
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}

	// 2. Now that gfMul's tables are ready, precompute the generator polynomials.
	for d := 1; d <= maxDivisorDegree; d++ {
		div, err := computeDivisor(d)
		if err != nil {
			panic(err) // unreachable: degree is in range
		}
		divisorCache[d] = div
	}
}

// gfMultiplyBitwise multiplies two field elements via the Russian-peasant method.
// It does not depend on the log tables, so it is used to bootstrap them.
func gfMultiplyBitwise(x, y int) int {
	z := 0
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= ((y >> i) & 1) * x
	}
	return z
}

// gfMul multiplies two field elements via the log tables.
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// gfInv returns the multiplicative inverse of a (a must be non-zero).
func gfInv(a byte) byte {
	return gfExp[255-int(gfLog[a])]
}

// Divisor returns the Reed-Solomon generator polynomial of the given degree,
// served from the precomputed cache when in range.
func Divisor(degree int) ([]byte, error) {
	if degree >= 1 && degree <= maxDivisorDegree {
		return divisorCache[degree], nil
	}
	return computeDivisor(degree)
}

// computeDivisor derives the generator polynomial for the given degree, which
// must be between 1 and 255 inclusive.
func computeDivisor(degree int) ([]byte, error) {
	if degree < 1 || degree > 255 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidDegree, degree)
	}

	res := make([]byte, degree)
	res[degree-1] = 1

	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < len(res); j++ {
			res[j] = gfMul(res[j], root)
			if j+1 < len(res) {
				res[j] ^= res[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}
	return res, nil
}

// Remainder computes the Reed-Solomon ECC remainder of data given a generator
// polynomial (divisor). The returned slice has len(divisor) bytes.
func Remainder(data, divisor []byte) []byte {
	res := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i := 0; i < len(res); i++ {
			res[i] ^= gfMul(divisor[i], factor)
		}
	}
	return res
}

// Correct repairs, in place, a received codeword of the form [data... | ecc...]
// with eccLen ECC bytes, fixing up to eccLen/2 errors. It returns
// ErrUncorrectable (wrapped) if the codeword cannot be repaired.
func Correct(block []byte, eccLen int) error {
	// 1. Syndromes S_i = R(generator^i), i = 0 .. eccLen-1.
	syndromes := make([]byte, eccLen)
	hasError := false
	for i := 0; i < eccLen; i++ {
		s := eval(block, gfExp[i])
		syndromes[i] = s
		if s != 0 {
			hasError = true
		}
	}
	if !hasError {
		return nil // clean — the common fast-path case
	}

	// 2. Berlekamp-Massey: find the error-locator polynomial sigma(x).
	locator := berlekampMassey(syndromes)
	degree := len(locator) - 1

	// 3. Chien search: roots of sigma give error positions.
	positions := make([]int, 0, degree)
	for i := 0; i < len(block); i++ {
		if evalReversed(locator, gfInv(gfExp[i%255])) == 0 {
			positions = append(positions, len(block)-1-i)
		}
	}
	if len(positions) != degree || degree == 0 {
		return fmt.Errorf("%w: error count mismatch", ErrUncorrectable)
	}

	// 4. Forney: compute error magnitudes and apply them.
	if err := forneyCorrect(block, syndromes, locator, positions); err != nil {
		return err
	}

	// 5. Re-verify syndromes are now zero.
	for i := 0; i < eccLen; i++ {
		if eval(block, gfExp[i]) != 0 {
			return fmt.Errorf("%w: residual error after correction", ErrUncorrectable)
		}
	}
	return nil
}

// eval evaluates the codeword polynomial (block[0] is the highest-degree
// coefficient) at x using Horner's method.
func eval(block []byte, x byte) byte {
	var acc byte
	for _, c := range block {
		acc = gfMul(acc, x) ^ c
	}
	return acc
}

// evalReversed evaluates poly where poly[0] is the constant term at x.
func evalReversed(poly []byte, x byte) byte {
	var acc byte
	for i := len(poly) - 1; i >= 0; i-- {
		acc = gfMul(acc, x) ^ poly[i]
	}
	return acc
}

// berlekampMassey returns the error-locator polynomial sigma(x) with
// index 0 = constant term (sigma_0 = 1). Subtraction is XOR; division is
// multiply-by-inverse.
func berlekampMassey(syndromes []byte) []byte {
	c := []byte{1} // current locator C(x)
	b := []byte{1} // last locator before the most recent length change B(x)
	l := 0         // current register length
	m := 1         // steps since last length change
	bb := byte(1)  // last discrepancy when length changed

	for n := 0; n < len(syndromes); n++ {
		delta := syndromes[n]
		for i := 1; i <= l && i < len(c); i++ {
			delta ^= gfMul(c[i], syndromes[n-i])
		}

		if delta == 0 {
			m++
			continue
		}

		scale := gfMul(delta, gfInv(bb))
		scaled := make([]byte, m+len(b))
		for i := range b {
			scaled[i+m] = gfMul(b[i], scale)
		}

		if 2*l <= n {
			t := make([]byte, len(c))
			copy(t, c)
			c = xorPoly(c, scaled)
			b = t
			l = n + 1 - l
			bb = delta
			m = 1
		} else {
			c = xorPoly(c, scaled)
			m++
		}
	}
	return c
}

// xorPoly returns a XOR b (index 0 = constant term), zero-extending the shorter.
func xorPoly(a, b []byte) []byte {
	if len(b) > len(a) {
		a, b = b, a
	}
	res := make([]byte, len(a))
	copy(res, a)
	for i := range b {
		res[i] ^= b[i]
	}
	return res
}

// forneyCorrect computes error values via Forney's algorithm and XORs them into
// the block at the located error positions.
func forneyCorrect(block []byte, syndromes, locator []byte, positions []int) error {
	eccLen := len(syndromes)
	synPoly := make([]byte, eccLen)
	copy(synPoly, syndromes)
	omega := polyMul(synPoly, locator)
	if len(omega) > eccLen {
		omega = omega[:eccLen]
	}

	deriv := formalDerivative(locator)

	n := len(block)
	for _, pos := range positions {
		power := n - 1 - pos
		xi := gfExp[power%255]
		xiInv := gfInv(xi)

		num := evalReversed(omega, xiInv)
		den := evalReversed(deriv, xiInv)
		if den == 0 {
			return fmt.Errorf("%w: forney zero denominator", ErrUncorrectable)
		}
		mag := gfMul(xi, gfMul(num, gfInv(den)))
		block[pos] ^= mag
	}
	return nil
}

// polyMul multiplies two polynomials (index 0 = constant term).
func polyMul(a, b []byte) []byte {
	res := make([]byte, len(a)+len(b)-1)
	for i := range a {
		if a[i] == 0 {
			continue
		}
		for j := range b {
			res[i+j] ^= gfMul(a[i], b[j])
		}
	}
	return res
}

// formalDerivative returns d/dx of poly over GF(2): odd-power terms survive
// (coefficient unchanged), even-power terms vanish.
func formalDerivative(poly []byte) []byte {
	if len(poly) <= 1 {
		return []byte{0}
	}
	res := make([]byte, len(poly)-1)
	for i := 1; i < len(poly); i++ {
		if i%2 == 1 {
			res[i-1] = poly[i]
		} else {
			res[i-1] = 0
		}
	}
	return res
}

// CorrectErasures repairs, in place, a received codeword of the form
// [data... | ecc...] with eccLen ECC bytes, whose bytes at the positions in
// erasures are known to be unreliable. It corrects the erasures and up to
// e further errors anywhere, provided 2e + len(erasures) <= budget, where
// budget is at most eccLen; a caller keeps eccLen - budget bytes of
// redundancy unused against miscorrection. The positions changed are those
// erased, whatever their values, and those of the errors found. It returns
// ErrUncorrectable (wrapped) if no codeword is within the budget.
func CorrectErasures(block []byte, eccLen int, erasures []int, budget int) error {
	s := len(erasures)
	budget = min(budget, eccLen)
	if s > budget {
		return fmt.Errorf("%w: too many erasures", ErrUncorrectable)
	}
	n := len(block)
	syndromes := make([]byte, eccLen)
	hasError := false
	for i := 0; i < eccLen; i++ {
		syndromes[i] = eval(block, gfExp[i])
		if syndromes[i] != 0 {
			hasError = true
		}
	}
	if !hasError {
		return nil
	}

	// Erasure locator Γ(x) = Π (1 - X_j x), X_j = α^(n-1-pos).
	gamma := []byte{1}
	seen := make(map[int]bool, s)
	for _, pos := range erasures {
		if pos < 0 || pos >= n || seen[pos] {
			return fmt.Errorf("%w: erasure position %d out of range or repeated", ErrUncorrectable, pos)
		}
		seen[pos] = true
		gamma = polyMul(gamma, []byte{1, gfExp[(n-1-pos)%255]})
	}

	// Forney syndromes Ξ = Γ·S mod x^eccLen; the error locator Λ follows
	// from Ξ_s .. Ξ_(eccLen-1) as from plain syndromes.
	xi := polyMul(gamma, syndromes)[:eccLen]
	lambda := berlekampMassey(xi[s:])
	for len(lambda) > 1 && lambda[len(lambda)-1] == 0 {
		lambda = lambda[:len(lambda)-1]
	}
	errs := len(lambda) - 1
	if 2*errs+s > budget {
		return fmt.Errorf("%w: beyond the correction budget", ErrUncorrectable)
	}

	// Λ and Γ end in nonzero coefficients, so Ψ = ΛΓ does: its degree is
	// the number of positions to correct.
	psi := polyMul(lambda, gamma)
	positions := make([]int, 0, len(psi)-1)
	for i := 0; i < n; i++ {
		if evalReversed(psi, gfInv(gfExp[i%255])) == 0 {
			positions = append(positions, n-1-i)
		}
	}
	if len(positions) != len(psi)-1 {
		return fmt.Errorf("%w: locator roots outside the codeword", ErrUncorrectable)
	}
	if err := forneyCorrect(block, syndromes, psi, positions); err != nil {
		return err
	}
	for i := 0; i < eccLen; i++ {
		if eval(block, gfExp[i]) != 0 {
			return fmt.Errorf("%w: residual error after correction", ErrUncorrectable)
		}
	}
	return nil
}
