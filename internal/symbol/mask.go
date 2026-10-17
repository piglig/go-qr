package symbol

import (
	"fmt"
	"math/bits"

	"github.com/piglig/go-qr/v2/internal/errs"
)

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
func (q *Builder) ApplyMask(msk int) error {
	if msk < 0 || msk > 7 {
		return fmt.Errorf("%w: mask %d out of range [0,7]", errs.ErrInvalidArgument, msk)
	}

	for y := 0; y < q.Size; y++ {
		for x := 0; x < q.Size; x++ {
			invert := maskInvert(msk, x, y)
			q.Modules[y][x] = q.Modules[y][x] != (invert && !q.IsFunction[y][x])
		}
	}
	return nil
}

// moduleLine is one row or column of a symbol: bit x is set when module x
// is dark. A symbol is at most 177 modules wide, which fits three words; the
// bits past its side are 0.
type moduleLine [3]uint64

// packedModules holds the modules of a symbol as rows and as columns, so
// that the mask penalty reads both directions a word at a time.
type packedModules struct {
	size       int
	rows, cols []moduleLine
}

func newPackedModules(size int) packedModules {
	return packedModules{size: size, rows: make([]moduleLine, size), cols: make([]moduleLine, size)}
}

// transposeRows sets the columns from the rows, 64×64 blocks at a time.
func (p *packedModules) transposeRows() {
	n := p.size
	var blk [64]uint64
	for bi := 0; bi*64 < n; bi++ { // rows bi*64 to bi*64+63
		for bj := 0; bj*64 < n; bj++ { // their word bj
			for k := range blk {
				blk[k] = 0
				if y := bi*64 + k; y < n {
					blk[k] = p.rows[y][bj]
				}
			}
			transpose64(&blk)
			for k := 0; k < 64 && bj*64+k < n; k++ {
				p.cols[bj*64+k][bi] = blk[k]
			}
		}
	}
}

// transpose64 transposes a 64×64 bit matrix in place: bit j of a[i] moves
// to bit i of a[j]. It swaps the off-diagonal 32×32 blocks, then the
// 16×16 blocks within each quarter, and so on down to single bits.
func transpose64(a *[64]uint64) {
	m := uint64(0x00000000FFFFFFFF)
	for j := 32; j != 0; j, m = j>>1, m^m<<(j>>1) {
		for k := 0; k < 64; k = (k + j + 1) &^ j {
			t := (a[k]>>j ^ a[k+j]) & m
			a[k+j] ^= t
			a[k] ^= t << j
		}
	}
}

// set sets module (x, y) in both the rows and the columns.
func (p *packedModules) set(x, y int, dark bool) {
	bx, by := uint64(1)<<(x&63), uint64(1)<<(y&63)
	if dark {
		p.rows[y][x>>6] |= bx
		p.cols[x][y>>6] |= by
	} else {
		p.rows[y][x>>6] &^= bx
		p.cols[x][y>>6] &^= by
	}
}

// xor sets p to the modules of a with those set in b inverted.
func (p *packedModules) xor(a, b *packedModules) {
	for i := range p.rows {
		for w := range p.rows[i] {
			p.rows[i][w] = a.rows[i][w] ^ b.rows[i][w]
			p.cols[i][w] = a.cols[i][w] ^ b.cols[i][w]
		}
	}
}

func b2u(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// sr and sl shift a line right and left by 0 < k < 64 across a word
// boundary: bit x of sr(cur, next, k) is bit x+k of the line, and bit x of
// sl(prev, cur, k) is bit x-k.
func sr(cur, next uint64, k uint) uint64 { return cur>>k | next<<(64-k) }
func sl(prev, cur uint64, k uint) uint64 { return cur<<k | prev>>(64-k) }

// penalty returns the penalty score of ISO/IEC 18004:2015 §7.8.3.1 for the
// modules in p, as the mask selection compares them:
//
//   - N1: each row or column run of L >= 5 modules of one color scores L-2;
//   - N2: each 2×2 block of one color scores 3, blocks overlapping;
//   - N3: each finder-like pattern scores 40. It is a sequence of whole runs
//     light a, dark n, light n, dark 3n, light n, dark n, light b in a row
//     or column, n >= 1, the area outside the symbol counting as light. It
//     scores once if a >= 4n and b >= n and once more if b >= 4n and
//     a >= n, so a pattern with 4n light modules on both sides scores 80;
//   - N4: 10 for each full 5% by which the proportion of dark modules
//     departs from 50%.
//
// Scores only grow, so once the score reaches limit the mask cannot win and
// penalty returns early with a score of at least limit.
func (p *packedModules) penalty(limit int) int {
	n := p.size
	words := (n + 63) / 64
	var pairs moduleLine // modules 0 to n-2, which have a next module
	for x := 0; x < n-1; x++ {
		pairs[x>>6] |= 1 << (x & 63)
	}
	score, dark := 0, 0
	for y := 0; y < n; y++ {
		r := &p.rows[y]
		score += linePenalty(r, words, &pairs)
		for i := 0; i < words; i++ {
			dark += bits.OnesCount64(r[i])
		}
		if y+1 == n {
			break
		}
		// N2: the blocks whose top-left module is in this row. Each module
		// matches the next one and the one below, and the next one the one
		// below it.
		below := &p.rows[y+1]
		for i := 0; i < words; i++ {
			var rNext, bNext uint64
			if i+1 < len(r) {
				rNext, bNext = r[i+1], below[i+1]
			}
			right, belowRight := sr(r[i], rNext, 1), sr(below[i], bNext, 1)
			same := ^(r[i] ^ right) &^ (r[i] ^ below[i]) &^ (right ^ belowRight)
			score += penaltyN2 * bits.OnesCount64(same&pairs[i])
		}
	}
	if score >= limit {
		return score
	}
	for x := 0; x < n; x++ {
		score += linePenalty(&p.cols[x], words, &pairs)
	}
	if score >= limit {
		return score
	}
	total := n * n
	k := (abs(dark*20-total*10)+total-1)/total - 1
	return score + k*penaltyN4
}

// linePenalty returns the N1 and N3 penalties of one row or column, read a
// word at a time. pairs has the bits of the modules that have a next module.
func linePenalty(d *moduleLine, words int, pairs *moduleLine) int {
	score := 0
	var prev, runTop uint64
	for i := 0; i < words; i++ {
		cur := d[i]
		var next, nextPairs uint64
		if i+1 < len(d) {
			next, nextPairs = d[i+1], pairs[i+1]
		}

		// N1: same has bit x when modules x and x+1 match, and run bit x
		// when modules x to x+4 do. A run of L >= 5 modules sets L-4
		// consecutive bits of run; it scores N1 + L-5, which is those bits
		// and N1-1 for the block.
		same := ^(cur ^ sr(cur, next, 1)) & pairs[i]
		sameNext := ^(next ^ next>>1) & nextPairs // only its low bits are read
		run := same & sr(same, sameNext, 1) & sr(same, sameNext, 2) & sr(same, sameNext, 3)
		starts := run &^ (run<<1 | runTop)
		runTop = run >> 63
		score += bits.OnesCount64(run) + (penaltyN1-1)*bits.OnesCount64(starts)

		// N3 with n = 1: from module x the line reads dark, light, dark,
		// dark, dark, light, dark, with light at x-1 and x+7 so that the
		// runs are whole. Bits shifted in from past either end of the line
		// are 0, the light area around the symbol.
		before := sl(prev, cur, 1)
		d1, d2, d3, d4, d5 := sr(cur, next, 1), sr(cur, next, 2), sr(cur, next, 3), sr(cur, next, 4), sr(cur, next, 5)
		d6, d7 := sr(cur, next, 6), sr(cur, next, 7)
		if core := cur & d2 & d3 & d4 & d6 &^ (before | d1 | d5 | d7); core != 0 {
			darkBefore := before | sl(prev, cur, 2) | sl(prev, cur, 3) | sl(prev, cur, 4)
			darkAfter := d7 | sr(cur, next, 8) | sr(cur, next, 9) | sr(cur, next, 10)
			score += penaltyN3 * (bits.OnesCount64(core&^darkBefore) + bits.OnesCount64(core&^darkAfter))
		}

		// N3 with n >= 2: the middle run, 3n >= 6 dark modules, follows at
		// least two light ones. Few runs start so; check each.
		for g := cur & d1 & d2 & d3 & d4 & d5 &^ (before | sl(prev, cur, 2)); g != 0; g &= g - 1 {
			score += scaledFinderPenalty(d, i<<6+bits.TrailingZeros64(g))
		}
		prev = cur
	}
	return score
}

// scaledFinderPenalty returns the N3 penalty of the finder-like pattern
// with n >= 2 whose middle run starts at module x of d, if there is one.
func scaledFinderPenalty(d *moduleLine, x int) int {
	c := d.darkRun(x)
	if c%3 != 0 {
		return 0
	}
	k := c / 3
	y := x + c // the first module after the middle run
	if !d.all(x-k, x, false) || !d.all(x-2*k, x-k, true) || d.dark(x-2*k-1) ||
		!d.all(y, y+k, false) || !d.all(y+k, y+2*k, true) || d.dark(y+2*k) {
		return 0
	}
	// The light runs on either side end at x-2k-1 and start at y+2k.
	score := 0
	if d.all(x-6*k, x-2*k, false) && d.all(y+2*k, y+3*k, false) {
		score += penaltyN3
	}
	if d.all(y+2*k, y+6*k, false) && d.all(x-3*k, x-2*k, false) {
		score += penaltyN3
	}
	return score
}

// dark reports whether module x is dark; modules outside the line are light.
func (d *moduleLine) dark(x int) bool {
	return x >= 0 && x < 64*len(d) && d[x>>6]>>(x&63)&1 != 0
}

// all reports whether modules a to b-1 are all dark, or all light.
func (d *moduleLine) all(a, b int, dark bool) bool {
	for x := a; x < b; x++ {
		if d.dark(x) != dark {
			return false
		}
	}
	return true
}

// darkRun returns the number of consecutive dark modules from x.
func (d *moduleLine) darkRun(x int) int {
	off := x & 63
	// The high off bits of the first word's light mask are past the word
	// and read as dark; the run then continues into the next word.
	if z := bits.TrailingZeros64(^d[x>>6] >> off); z < 64-off {
		return z
	}
	n := 64 - off
	for i := x>>6 + 1; i < len(d); i++ {
		z := bits.TrailingZeros64(^d[i])
		n += z
		if z < 64 {
			break
		}
	}
	return n
}
