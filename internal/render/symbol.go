// Package render draws QR Code symbols for package qr: plain and styled
// rasters, PNG written straight from the modules, SVG paths, gradients,
// finder colors and logos. Package qr validates the options and the logo's
// error correction budget, and calls these with its symbol's modules.
package render

import "image"

// Symbol is the module grid of a symbol to draw.
type Symbol struct {
	N       int      // side in modules
	Modules [][]bool // Modules[y][x] reports a dark module
}

// Size returns the side of the symbol in modules.
func (s *Symbol) Size() int { return s.N }

// Module reports whether module (x, y) is dark; modules outside the symbol
// are light, as the quiet zone is.
func (s *Symbol) Module(x, y int) bool {
	return 0 <= x && x < s.N && 0 <= y && y < s.N && s.Modules[y][x]
}

// ModuleShape and FinderShape are package qr's shapes, numbered alike.
type (
	ModuleShape int
	FinderShape int
)

// The shapes.
const (
	ModuleSquare ModuleShape = iota
	ModuleDot
	ModuleRounded
)

const (
	FinderSquare FinderShape = iota
	FinderRounded
	FinderCircle
)

// Logo is an image drawn over the center of the symbol, ratio of its side.
type Logo struct {
	Img   image.Image
	Ratio float64
}
