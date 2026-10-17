package symbol

import (
	"math"
	"math/rand"
	"testing"
)

// referencePenalty is the penalty score of the encoder up to v2.6, which
// visits the grid module by module and finds finder-like patterns through
// a history of the last seven run lengths, the method of Project Nayuki's
// QR Code generator. packedModules.penalty must agree with it on every grid.
func referencePenalty(grid [][]bool) int {
	q := &Builder{Size: len(grid)}
	return q.referencePenaltyScore(grid, math.MaxInt32)
}

// The functions below are those of mask.go up to v2.6, renamed.

// referencePenaltyScore calculates and returns a penalty score based on several criteria.
//
// cap is an early-abort threshold: once the partial score reaches cap it can no
// longer win the mask comparison (scores only grow), so we bail out and return a
// value >= cap. Pass math.MaxInt32 to force a full, exact computation.
func (q *Builder) referencePenaltyScore(grid [][]bool, cap int) int {
	res := 0
	size := q.Size
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
				q.refFinderPenaltyAddHistory(runX, runHistory)
				if !runColor {
					res += q.refFinderPenaltyCountPatterns(runHistory) * penaltyN3
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
		res += q.refFinderPenaltyTerminateAndCount(runColor, runX, runHistory) * penaltyN3
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
				q.refFinderPenaltyAddHistory(runY, runHistory)
				if !runColor {
					res += q.refFinderPenaltyCountPatterns(runHistory) * penaltyN3
				}
				runColor = cell
				runY = 1
			}
		}
		res += q.refFinderPenaltyTerminateAndCount(runColor, runY, runHistory) * penaltyN3
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

// refFinderPenaltyCountPatterns checks if patterns in runHistory follow the 1:1:3:1:1 finder ratio.
func (q *Builder) refFinderPenaltyCountPatterns(runHistory []int) int {
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

// refFinderPenaltyTerminateAndCount finalizes the run history and returns the residual finder penalty.
func (q *Builder) refFinderPenaltyTerminateAndCount(currentRunColor bool, currentRunLen int, runHistory []int) int {
	if currentRunColor {
		q.refFinderPenaltyAddHistory(currentRunLen, runHistory)
		currentRunLen = 0
	}
	currentRunLen += q.Size
	q.refFinderPenaltyAddHistory(currentRunLen, runHistory)
	return q.refFinderPenaltyCountPatterns(runHistory)
}

// refFinderPenaltyAddHistory shifts the run-length history and prepends the current run length.
func (q *Builder) refFinderPenaltyAddHistory(currentRunLen int, runHistory []int) {
	if runHistory[0] == 0 {
		currentRunLen += q.Size
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

// pack sets p to the modules of grid, which must have p's size.
func (p *packedModules) pack(grid [][]bool) {
	for y, row := range grid {
		var line moduleLine
		for i := 0; i*64 < len(row); i++ {
			var w uint64
			for k, dark := range row[i*64 : min(len(row), i*64+64)] {
				w |= b2u(dark) << k
			}
			line[i] = w
		}
		p.rows[y] = line
	}
	p.transposeRows()
}

func packedPenalty(grid [][]bool) int {
	p := newPackedModules(len(grid))
	p.pack(grid)
	return p.penalty(math.MaxInt32)
}

func randomGrid(rng *rand.Rand, size int, density float64) [][]bool {
	g := make([][]bool, size)
	for y := range g {
		g[y] = make([]bool, size)
		for x := range g[y] {
			g[y][x] = rng.Float64() < density
		}
	}
	return g
}

// runGrid fills rows with random runs whose lengths favor scaled
// finder-like patterns, transposed half of the time.
func runGrid(rng *rand.Rand, size int) [][]bool {
	lens := []int{1, 1, 2, 2, 3, 3, 4, 6, 8, 9, 12, 15}
	g := make([][]bool, size)
	for y := range g {
		g[y] = make([]bool, size)
		x, dark := 0, rng.Intn(2) == 0
		for x < size {
			for i := lens[rng.Intn(len(lens))]; i > 0 && x < size; i-- {
				g[y][x] = dark
				x++
			}
			dark = !dark
		}
	}
	if rng.Intn(2) == 0 {
		g = transposeGrid(g)
	}
	return g
}

func transposeGrid(g [][]bool) [][]bool {
	t := make([][]bool, len(g))
	for y := range t {
		t[y] = make([]bool, len(g))
		for x := range t[y] {
			t[y][x] = g[x][y]
		}
	}
	return t
}

// plantFinder writes light a, dark n, light n, dark 3n, light n, dark n,
// light b into row y from x0, and a dark module after it.
func plantFinder(g [][]bool, y, x0, n, a, b int) {
	x := x0
	for i, l := range []int{a, n, n, 3 * n, n, n, b} {
		for ; l > 0 && x < len(g); l-- {
			g[y][x] = i%2 == 1
			x++
		}
	}
	if x < len(g) {
		g[y][x] = true
	}
}

func TestPenaltyMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	check := func(name string, g [][]bool) {
		t.Helper()
		if got, want := packedPenalty(g), referencePenalty(g); got != want {
			t.Fatalf("%s, size %d: penalty %d, want %d", name, len(g), got, want)
		}
	}
	for v := 1; v <= 40; v++ {
		size := 4*v + 17
		for _, density := range []float64{0.5, 0.1, 0.9, 0.3} {
			for i := 0; i < 10; i++ {
				check("random", randomGrid(rng, size, density))
			}
		}
		for i := 0; i < 30; i++ {
			check("runs", runGrid(rng, size))
		}
	}
	// Scaled patterns with flanks around n and 4n, at both ends and inside
	// rows and columns, including sides at word boundaries.
	for _, size := range []int{21, 25, 57, 63, 64, 65, 101, 127, 128, 129, 177} {
		for n := 1; 7*n <= size; n++ {
			for _, ab := range [][2]int{{4 * n, n}, {n, 4 * n}, {4*n - 1, 4 * n}, {n - 1, 4 * n}, {0, 0}, {4 * n, 4 * n}, {size, size}} {
				for x0 := 0; x0+7*n <= size; x0 += 1 + size/9 {
					g := randomGrid(rng, size, 0.5)
					plantFinder(g, size/3, x0, n, ab[0], ab[1])
					check("planted in a row", g)
					check("planted in a column", transposeGrid(g))
				}
			}
		}
	}
	for _, g := range [][][]bool{randomGrid(rng, 21, 0), randomGrid(rng, 21, 1), randomGrid(rng, 177, 0), randomGrid(rng, 177, 1)} {
		check("uniform", g)
	}
}

func TestPenaltyLimit(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	g := randomGrid(rng, 57, 0.5)
	p := newPackedModules(57)
	p.pack(g)
	full := p.penalty(math.MaxInt32)
	for _, limit := range []int{0, 1, full / 2, full, full + 1} {
		got := p.penalty(limit)
		if got < full && got < limit || limit > full && got != full {
			t.Errorf("limit %d: penalty %d, full score %d", limit, got, full)
		}
	}
}

func TestTranspose64(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	var a, b [64]uint64
	for i := range a {
		a[i] = rng.Uint64()
	}
	b = a
	transpose64(&b)
	for i := 0; i < 64; i++ {
		for j := 0; j < 64; j++ {
			if a[i]>>j&1 != b[j]>>i&1 {
				t.Fatalf("bit %d of row %d is not bit %d of row %d", j, i, i, j)
			}
		}
	}
}

func TestPackedModules(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for _, size := range []int{21, 63, 64, 65, 129, 177} {
		g := randomGrid(rng, size, 0.5)
		p := newPackedModules(size)
		p.pack(g)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if p.rows[y].dark(x) != g[y][x] || p.cols[x].dark(y) != g[y][x] {
					t.Fatalf("size %d: module (%d, %d) packed wrong", size, x, y)
				}
			}
			if p.rows[y].dark(size) || p.cols[y].dark(size) {
				t.Fatalf("size %d: bit past the side set", size)
			}
		}
		x, y := rng.Intn(size), rng.Intn(size)
		p.set(x, y, !g[y][x])
		if p.rows[y].dark(x) == g[y][x] || p.cols[x].dark(y) == g[y][x] {
			t.Fatalf("size %d: set (%d, %d) did not change both", size, x, y)
		}
	}
}

func TestDarkRun(t *testing.T) {
	var d moduleLine
	for x := 60; x < 140; x++ { // across both word boundaries
		d[x>>6] |= 1 << (x & 63)
	}
	for _, tc := range [][2]int{{60, 80}, {63, 77}, {64, 76}, {100, 40}, {139, 1}, {140, 0}, {0, 0}} {
		if got := d.darkRun(tc[0]); got != tc[1] {
			t.Errorf("darkRun(%d) = %d, want %d", tc[0], got, tc[1])
		}
	}
	all := moduleLine{^uint64(0), ^uint64(0), ^uint64(0)}
	if got := all.darkRun(5); got != 187 {
		t.Errorf("darkRun of a full line = %d, want 187", got)
	}
}

// referenceMask chooses the mask of b's symbol with referencePenalty.
func referenceMask(b *Builder) int {
	best, minPenalty := 0, math.MaxInt32
	grid := make([][]bool, b.Size)
	for y := range grid {
		grid[y] = make([]bool, b.Size)
	}
	for m := 0; m < 8; m++ {
		b.DrawFormatBits(m)
		for y := range grid {
			for x := range grid[y] {
				grid[y][x] = b.Modules[y][x] != (maskInvert(m, x, y) && !b.IsFunction[y][x])
			}
		}
		if p := referencePenalty(grid); p < minPenalty {
			best, minPenalty = m, p
		}
	}
	return best
}

func TestChooseBestMaskMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 300; i++ {
		ver, ecc := rng.Intn(MaxVersion)+1, Level(rng.Intn(4))
		data := make([]byte, NumDataCodewords(ver, ecc))
		rng.Read(data)
		b := NewBuilder(ver, ecc)
		b.DrawFunctionPatterns()
		raw, err := b.AddECCAndInterleave(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := b.DrawCodewords(raw); err != nil {
			t.Fatal(err)
		}
		got := b.ChooseBestMask()
		if want := referenceMask(b); got != want {
			t.Fatalf("version %d-%d: mask %d, want %d", ver, ecc, got, want)
		}
	}
}

func FuzzMaskPenalty(f *testing.F) {
	f.Add(uint8(0), false, []byte{0x5a, 0xc3})
	f.Add(uint8(39), true, []byte{1, 1, 3, 1, 1, 4, 2, 2, 6, 2, 2, 8})
	f.Fuzz(func(t *testing.T, ver uint8, runs bool, data []byte) {
		if len(data) == 0 {
			return
		}
		size := 4*(int(ver)%40+1) + 17
		g := make([][]bool, size)
		for y := range g {
			g[y] = make([]bool, size)
		}
		// The data gives module bits, or with runs, run lengths of 1 to 16
		// in alternating colors, both in reading order and repeated.
		i, dark, left := 0, false, 0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if runs {
					for left == 0 {
						left = int(data[i%len(data)])%16 + 1
						dark = !dark
						i++
					}
					g[y][x] = dark
					left--
				} else {
					k := y*size + x
					g[y][x] = data[k/8%len(data)]>>(k%8)&1 != 0
				}
			}
		}
		if got, want := packedPenalty(g), referencePenalty(g); got != want {
			t.Fatalf("size %d: penalty %d, want %d", size, got, want)
		}
		if got, want := packedPenalty(transposeGrid(g)), referencePenalty(transposeGrid(g)); got != want {
			t.Fatalf("size %d transposed: penalty %d, want %d", size, got, want)
		}
	})
}
