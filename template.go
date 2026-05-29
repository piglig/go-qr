package go_qr

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
	// maskPatterns[m][y][x] reports whether mask m flips module (x, y); function
	// modules are always false (the mask never touches them). Each grid's rows
	// alias one backing allocation.
	maskPatterns [8][][]bool
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
	b := newBuilder(version, Low)
	b.drawFunctionPatterns()

	t := &qrTemplate{}
	for m := 0; m < 8; m++ {
		grid := make([][]bool, size)
		backing := make([]bool, size*size)
		for y := 0; y < size; y++ {
			grid[y] = backing[y*size : (y+1)*size]
			fn := b.isFunction[y]
			for x := 0; x < size; x++ {
				grid[y][x] = maskInvert(m, x, y) && !fn[x]
			}
		}
		t.maskPatterns[m] = grid
	}

	actual, _ := templateCache.LoadOrStore(version, t)
	return actual.(*qrTemplate)
}
