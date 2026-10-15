package qr

import "sync"

// qrTemplate holds version-invariant data that can be reused across every encode
// of the same version. The expensive, repeatedly-recomputed part of encoding is
// the per-mask grid maskInvert(mask, x, y) && !isFunction[y][x]: it depends only
// on the version (through the function-pattern map) and the mask number, never
// on the payload. Precomputing the eight mask patterns once per version turns
// the inner mask-selection loop's grid build into a plain XOR and removes the
// per-module maskInvert evaluation, which profiling showed dominates encoding of
// small/medium symbols.
type qrTemplate struct {
	// maskPatterns[m] has the modules mask m flips, as rows and columns;
	// function modules are never set (the mask never touches them).
	maskPatterns [8]packedModules

	// bitAt[y][x] is the index, in placement order, of the data bit that
	// module (x, y) carries, or -1 for function modules and remainder bits;
	// the bit belongs to raw codeword bitAt[y][x]/8.
	bitAt [][]int16

	// functionRows has the function patterns packed by row, with the format
	// bits of mask 0 at level L; the other modules are light.
	functionRows []moduleLine

	// pilot[y*size+x] reports whether module (x, y) is known before reading
	// the symbol: a function module other than the format information,
	// which depends on the level and mask. pilotDark is its value.
	pilot, pilotDark []bool
}

// templateCache maps version -> *qrTemplate. Templates are immutable once built,
// so concurrent encodes (e.g. EncodeBatch) share them without further locking.
var templateCache sync.Map

// getTemplate returns the cached template for version, building it on first use.
func getTemplate(version int) *qrTemplate {
	if v, ok := templateCache.Load(version); ok {
		return v.(*qrTemplate)
	}

	size := version*4 + 17
	// A throwaway builder gives us the version's function-pattern map; only
	// isFunction is read here (module values are irrelevant).
	b := newBuilder(version, ECCLow)
	b.drawFunctionPatterns()

	t := &qrTemplate{}
	for m := 0; m < 8; m++ {
		p := newPackedModules(size)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if maskInvert(m, x, y) && !b.isFunction[y][x] {
					p.set(x, y, true)
				}
			}
		}
		t.maskPatterns[m] = p
	}

	t.bitAt = bitMap(b)
	t.pilot, t.pilotDark = make([]bool, size*size), make([]bool, size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if b.isFunction[y][x] && !isFormatModule(x, y, size) {
				t.pilot[y*size+x], t.pilotDark[y*size+x] = true, b.modules[y][x]
			}
		}
	}
	t.functionRows = make([]moduleLine, size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if b.modules[y][x] {
				t.functionRows[y][x>>6] |= 1 << (x & 63)
			}
		}
	}

	actual, _ := templateCache.LoadOrStore(version, t)
	return actual.(*qrTemplate)
}

// bitMap walks the codeword placement of ISO/IEC 18004 §7.7.3 over the
// builder's function-pattern map, recording which data bit each module
// carries: two-module-wide columns from the right, alternately upward and
// downward, skipping the vertical timing pattern.
func bitMap(b *builder) [][]int16 {
	size := b.size
	n := numRawDataModules(b.version) / 8 * 8
	out := make([][]int16, size)
	backing := make([]int16, size*size)
	for y := range out {
		out[y] = backing[y*size : (y+1)*size]
		for x := range out[y] {
			out[y][x] = -1
		}
	}
	i := 0
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		upward := (right+1)&2 == 0
		for vert := 0; vert < size; vert++ {
			y := vert
			if upward {
				y = size - 1 - vert
			}
			for j := 0; j < 2; j++ {
				if x := right - j; !b.isFunction[y][x] && i < n {
					out[y][x] = int16(i)
					i++
				}
			}
		}
	}
	return out
}
