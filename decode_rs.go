package go_qr

import "fmt"

// Reed-Solomon decoding over GF(2^8) with the QR Code field polynomial 0x11D.
//
// The encoder side (reedsolomon.go) only needs polynomial division to produce
// ECC bytes. Decoding needs the inverse problem: given a received codeword that
// may contain errors, recover the original. That requires syndrome evaluation,
// the Berlekamp-Massey algorithm (error-locator polynomial), a Chien search
// (error positions), and Forney's algorithm (error magnitudes).
//
// The log/antilog tables are derived once from the existing reedSolomonMultiply
// so there is a single source of truth for the field arithmetic.

var (
	gfExp [512]byte // gfExp[i] = generator^i (doubled length to avoid mod on multiply)
	gfLog [256]byte // gfLog[x] = i such that generator^i = x
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x = reedSolomonMultiply(x, 0x02)
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
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

// rsCorrect corrects in place a received codeword block of the form
// [data... | ecc...] with eccLen ECC bytes, using up to eccLen/2 error
// corrections. It returns an error if the block is uncorrectable.
func rsCorrect(block []byte, eccLen int) error {
	// 1. Syndromes S_i = R(generator^i), i = 0 .. eccLen-1.
	syndromes := make([]byte, eccLen)
	hasError := false
	for i := 0; i < eccLen; i++ {
		s := rsEval(block, gfExp[i])
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
		// Evaluate sigma at generator^-i; a root means position i has an error.
		if rsEvalReversed(locator, gfInv(gfExp[i%255])) == 0 {
			// position from the high-order end of the codeword
			positions = append(positions, len(block)-1-i)
		}
	}
	if len(positions) != degree || degree == 0 {
		return fmt.Errorf("%w: reed-solomon error count mismatch", ErrDecodeFailed)
	}

	// 4. Forney: compute error magnitudes and apply them.
	if err := forneyCorrect(block, syndromes, locator, positions); err != nil {
		return err
	}

	// 5. Re-verify syndromes are now zero.
	for i := 0; i < eccLen; i++ {
		if rsEval(block, gfExp[i]) != 0 {
			return fmt.Errorf("%w: reed-solomon residual error", ErrDecodeFailed)
		}
	}
	return nil
}

// rsEval evaluates the codeword polynomial (block[0] is the highest-degree
// coefficient) at x using Horner's method.
func rsEval(block []byte, x byte) byte {
	var acc byte
	for _, c := range block {
		acc = gfMul(acc, x) ^ c
	}
	return acc
}

// rsEvalReversed evaluates poly where poly[0] is the constant term (the layout
// produced by berlekampMassey) at x.
func rsEvalReversed(poly []byte, x byte) byte {
	var acc byte
	for i := len(poly) - 1; i >= 0; i-- {
		acc = gfMul(acc, x) ^ poly[i]
	}
	return acc
}

// berlekampMassey returns the error-locator polynomial sigma(x) with
// index 0 = constant term (sigma_0 = 1), via the standard iteration over the
// syndromes. Subtraction is XOR; division is multiply-by-inverse.
func berlekampMassey(syndromes []byte) []byte {
	c := []byte{1} // current locator C(x)
	b := []byte{1} // last locator before the most recent length change B(x)
	l := 0         // current register length
	m := 1         // steps since last length change
	bb := byte(1)  // last discrepancy when length changed

	for n := 0; n < len(syndromes); n++ {
		// discrepancy delta = S[n] + sum_{i=1..l} C[i]*S[n-i]
		delta := syndromes[n]
		for i := 1; i <= l && i < len(c); i++ {
			delta ^= gfMul(c[i], syndromes[n-i])
		}

		if delta == 0 {
			m++
			continue
		}

		// scaled = (delta/bb) * x^m * B(x)
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
	// Error evaluator omega(x) = S(x) * sigma(x) mod x^eccLen.
	eccLen := len(syndromes)
	synPoly := make([]byte, eccLen) // synPoly[i] = S_i, index 0 = constant
	copy(synPoly, syndromes)
	omega := polyMul(synPoly, locator)
	if len(omega) > eccLen {
		omega = omega[:eccLen]
	}

	// sigma'(x): formal derivative of the locator (drop even-index terms).
	deriv := formalDerivative(locator)

	n := len(block)
	for _, pos := range positions {
		// X_i = generator^(power), where power corresponds to the position.
		power := n - 1 - pos
		xi := gfExp[power%255]
		xiInv := gfInv(xi)

		num := rsEvalReversed(omega, xiInv)
		den := rsEvalReversed(deriv, xiInv)
		if den == 0 {
			return fmt.Errorf("%w: forney zero denominator", ErrDecodeFailed)
		}
		// magnitude = X_i * omega(X_i^-1) / sigma'(X_i^-1)
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
