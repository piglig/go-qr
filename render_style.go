package qr

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"
)

// ModuleShape is the shape drawn for each dark module outside the finder
// patterns.
type ModuleShape int

const (
	// ModuleSquare fills the whole module; it is the default.
	ModuleSquare ModuleShape = iota
	// ModuleDot draws a circle 0.8 modules across.
	ModuleDot
	// ModuleRounded rounds every corner whose two neighbors are light, so
	// runs of dark modules merge into smooth shapes and isolated modules
	// become circles.
	ModuleRounded
)

func (s ModuleShape) valid() bool { return ModuleSquare <= s && s <= ModuleRounded }

// FinderShape is the shape of the three finder patterns in the corners.
type FinderShape int

const (
	// FinderSquare draws the standard square ring and center; it is the
	// default.
	FinderSquare FinderShape = iota
	// FinderRounded rounds the ring and the center.
	FinderRounded
	// FinderCircle draws a circular ring around a round center.
	FinderCircle
)

func (s FinderShape) valid() bool { return FinderSquare <= s && s <= FinderCircle }

// WithModuleShape sets the shape of the dark modules. Readers sample module
// centers, so every shape keeps the center dark; check unusual styles with
// Code.Verify before publishing. Text output ignores it.
func WithModuleShape(s ModuleShape) RenderOption {
	return func(c *renderConfig) { c.moduleShape = s }
}

// WithFinderShape sets the shape of the three finder patterns. Every shape
// keeps the 1:1:3:1:1 proportions that readers look for along the rows and
// columns through the center. Text output ignores it.
func WithFinderShape(s FinderShape) RenderOption {
	return func(c *renderConfig) { c.finderShape = s }
}

// styled reports whether the rendering differs from plain square modules in
// a single color.
func (c *renderConfig) styled() bool {
	return c.moduleShape != ModuleSquare || c.finderShape != FinderSquare
}

// dotRadius is the radius of a ModuleDot, in modules.
const dotRadius = 0.4

// Corner bits of a ModuleRounded module: set when the corner is rounded.
const (
	cornerTL = 1 << iota
	cornerTR
	cornerBR
	cornerBL
)

// roundedCorners returns which corners of the dark module at (x, y) have
// both orthogonal neighbors light.
func roundedCorners(c *Code, x, y int) int {
	up, down := c.Module(x, y-1), c.Module(x, y+1)
	left, right := c.Module(x-1, y), c.Module(x+1, y)
	mask := 0
	if !up && !left {
		mask |= cornerTL
	}
	if !up && !right {
		mask |= cornerTR
	}
	if !down && !right {
		mask |= cornerBR
	}
	if !down && !left {
		mask |= cornerBL
	}
	return mask
}

// finderOrigin reports whether module (x, y) lies in one of the 7×7 finder
// patterns of a symbol of the given size, and returns that finder's
// top-left module.
func finderOrigin(x, y, size int) (fx, fy int, ok bool) {
	for _, o := range [3][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
		if x >= o[0] && x < o[0]+7 && y >= o[1] && y < o[1]+7 {
			return o[0], o[1], true
		}
	}
	return 0, 0, false
}

// insideModule reports whether point (u, v), in module units within the
// module's box [0,1)², is covered by a module of the given shape and corner
// mask.
func insideModule(shape ModuleShape, corners int, u, v float64) bool {
	switch shape {
	case ModuleDot:
		return (u-0.5)*(u-0.5)+(v-0.5)*(v-0.5) <= dotRadius*dotRadius
	case ModuleRounded:
		const r = 0.5
		var cx, cy float64
		switch {
		case u < r && v < r && corners&cornerTL != 0:
			cx, cy = r, r
		case u >= 1-r && v < r && corners&cornerTR != 0:
			cx, cy = 1-r, r
		case u >= 1-r && v >= 1-r && corners&cornerBR != 0:
			cx, cy = 1-r, 1-r
		case u < r && v >= 1-r && corners&cornerBL != 0:
			cx, cy = r, 1-r
		default:
			return true
		}
		return (u-cx)*(u-cx)+(v-cy)*(v-cy) <= r*r
	}
	return true
}

// finderGeometry describes a finder pattern as three nested shapes, in
// module units within its 7×7 box: the ring is outer minus hole.
type finderGeometry struct {
	outer, hole, center roundedBox
}

// roundedBox is a square at (x, y) with side w and corner radius r; r equal
// to w/2 makes it a circle.
type roundedBox struct{ x, y, w, r float64 }

func (b roundedBox) contains(u, v float64) bool {
	if u < b.x || v < b.y || u >= b.x+b.w || v >= b.y+b.w {
		return false
	}
	// Distance from the nearest corner center, when in a corner region.
	cx := math.Max(b.x+b.r, math.Min(u, b.x+b.w-b.r))
	cy := math.Max(b.y+b.r, math.Min(v, b.y+b.w-b.r))
	return (u-cx)*(u-cx)+(v-cy)*(v-cy) <= b.r*b.r
}

func finderShapeGeometry(s FinderShape) finderGeometry {
	switch s {
	case FinderRounded:
		return finderGeometry{roundedBox{0, 0, 7, 2}, roundedBox{1, 1, 5, 1}, roundedBox{2, 2, 3, 1}}
	case FinderCircle:
		return finderGeometry{roundedBox{0, 0, 7, 3.5}, roundedBox{1, 1, 5, 2.5}, roundedBox{2, 2, 3, 1.5}}
	}
	return finderGeometry{roundedBox{0, 0, 7, 0}, roundedBox{1, 1, 5, 0}, roundedBox{2, 2, 3, 0}}
}

// supersample is the number of samples per pixel side when computing shape
// coverage for anti-aliasing.
const supersample = 4

// coverageTile returns the coverage, 0-255, of each pixel in a tile of
// side px pixels spanning units module units, by inside.
func coverageTile(px int, units float64, inside func(u, v float64) bool) []uint8 {
	tile := make([]uint8, px*px)
	step := units / float64(px*supersample)
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			n := 0
			for sy := 0; sy < supersample; sy++ {
				v := (float64(y*supersample+sy) + 0.5) * step
				for sx := 0; sx < supersample; sx++ {
					if inside((float64(x*supersample+sx)+0.5)*step, v) {
						n++
					}
				}
			}
			tile[y*px+x] = uint8(n * 255 / (supersample * supersample))
		}
	}
	return tile
}

// tileKey identifies a cached coverage tile.
type tileKey struct {
	kind   int // 0: module, 1: finder
	shape  int
	detail int // corner mask for modules
	scale  int
}

// tileCache holds coverage tiles, which depend only on the shape and scale,
// so repeated renders skip the supersampling.
var tileCache sync.Map // tileKey -> []uint8

func cachedTile(k tileKey, px int, units float64, inside func(u, v float64) bool) []uint8 {
	if t, ok := tileCache.Load(k); ok {
		return t.([]uint8)
	}
	t, _ := tileCache.LoadOrStore(k, coverageTile(px, units, inside))
	return t.([]uint8)
}

// moduleTile returns the coverage of one dark module.
func moduleTile(shape ModuleShape, corners, scale int) []uint8 {
	return cachedTile(tileKey{0, int(shape), corners, scale}, scale, 1, func(u, v float64) bool {
		return insideModule(shape, corners, u, v)
	})
}

// finderTile returns the coverage of a whole finder pattern, ring and
// center.
func finderTile(shape FinderShape, scale int) []uint8 {
	g := finderShapeGeometry(shape)
	return cachedTile(tileKey{1, int(shape), 0, scale}, 7*scale, 7, func(u, v float64) bool {
		return g.center.contains(u, v) || g.outer.contains(u, v) && !g.hole.contains(u, v)
	})
}

// paintCoverage renders code with the configured shapes as an alpha map of
// side×side pixels: 0 is background, 255 is fully dark. Module cells and
// finder boxes do not overlap, so tiles are copied rather than blended.
func paintCoverage(code *Code, c *renderConfig, side int) []uint8 {
	cov := make([]uint8, side*side)
	s, size, qz := c.scale, code.Size(), c.quietZone
	copyTile := func(tile []uint8, px, x0, y0 int) {
		for ty := 0; ty < px; ty++ {
			copy(cov[(y0+ty)*side+x0:], tile[ty*px:(ty+1)*px])
		}
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !code.Module(x, y) {
				continue
			}
			if _, _, ok := finderOrigin(x, y, size); ok {
				continue
			}
			corners := 0
			if c.moduleShape == ModuleRounded {
				corners = roundedCorners(code, x, y)
			}
			copyTile(moduleTile(c.moduleShape, corners, s), s, (x+qz)*s, (y+qz)*s)
		}
	}
	finder := finderTile(c.finderShape, s)
	for _, o := range [3][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
		copyTile(finder, 7*s, (o[0]+qz)*s, (o[1]+qz)*s)
	}
	return cov
}

// blendPalette returns the 256 colors between bg (index 0) and fg (index
// 255), so a coverage map indexes it directly.
func blendPalette(fg, bg color.Color) [256]color.RGBA {
	f, b := rgba(fg), rgba(bg)
	var p [256]color.RGBA
	for i := range p {
		a, ia := uint32(i), 255-uint32(i)
		p[i] = color.RGBA{
			R: uint8((uint32(f.R)*a + uint32(b.R)*ia + 127) / 255),
			G: uint8((uint32(f.G)*a + uint32(b.G)*ia + 127) / 255),
			B: uint8((uint32(f.B)*a + uint32(b.B)*ia + 127) / 255),
			A: uint8((uint32(f.A)*a + uint32(b.A)*ia + 127) / 255),
		}
	}
	return p
}

// paintStyled renders code with the configured shapes into a new RGBA
// image.
func paintStyled(code *Code, c *renderConfig, side int) *image.RGBA {
	cov := paintCoverage(code, c, side)
	pal := blendPalette(c.fg, c.bg)
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for i, a := range cov {
		p := pal[a]
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = p.R, p.G, p.B, p.A
	}
	return img
}

// paintStyledPaletted renders code with the configured shapes into an
// 8-bit paletted image, which encodes to a much smaller and faster PNG than
// RGBA: every pixel is a blend of the two colors.
func paintStyledPaletted(code *Code, c *renderConfig, side int) *image.Paletted {
	pal := blendPalette(c.fg, c.bg)
	palette := make(color.Palette, len(pal))
	for i := range pal {
		palette[i] = pal[i]
	}
	return &image.Paletted{Pix: paintCoverage(code, c, side), Stride: side, Rect: image.Rect(0, 0, side, side), Palette: palette}
}

// writeStyledSVGBody writes the modules and finder patterns of code with the
// configured shapes.
func (c *Code) writeStyledSVGBody(sb *strings.Builder, cfg *renderConfig) {
	s := float64(cfg.scale)
	qz := float64(cfg.quietZone)
	size := c.Size()
	fill := colorToSVG(cfg.fg)

	var d svgPath
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !c.Module(x, y) {
				continue
			}
			if _, _, ok := finderOrigin(x, y, size); ok {
				continue
			}
			px, py := (float64(x)+qz)*s, (float64(y)+qz)*s
			switch cfg.moduleShape {
			case ModuleDot:
				d.circle(px+s/2, py+s/2, dotRadius*s)
			case ModuleRounded:
				corners := roundedCorners(c, x, y)
				rad := func(bit int) float64 {
					if corners&bit != 0 {
						return s / 2
					}
					return 0
				}
				d.roundedSquare(px, py, s, rad(cornerTL), rad(cornerTR), rad(cornerBR), rad(cornerBL))
			default:
				d.roundedSquare(px, py, s, 0, 0, 0, 0)
			}
		}
	}
	sb.WriteString("\t<path d=\"")
	sb.Write(d.b)
	sb.WriteString("\" fill=\"" + fill + "\"/>\n")

	g := finderShapeGeometry(cfg.finderShape)
	for _, o := range [3][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
		ox, oy := (float64(o[0])+qz)*s, (float64(o[1])+qz)*s
		var ring, center svgPath
		ring.box(g.outer, ox, oy, s)
		ring.box(g.hole, ox, oy, s)
		center.box(g.center, ox, oy, s)
		sb.WriteString("\t<path d=\"")
		sb.Write(ring.b)
		sb.WriteString("\" fill=\"" + fill + "\" fill-rule=\"evenodd\"/>\n\t<path d=\"")
		sb.Write(center.b)
		sb.WriteString("\" fill=\"" + fill + "\"/>\n")
	}
}

// svgPath accumulates SVG path data without per-coordinate allocations.
type svgPath struct{ b []byte }

// num appends v with at most two decimals.
func (p *svgPath) num(v float64) {
	p.b = strconv.AppendFloat(p.b, math.Round(v*100)/100, 'f', -1, 64)
}

func (p *svgPath) cmd(c byte, vs ...float64) {
	p.b = append(p.b, c)
	for i, v := range vs {
		if i > 0 {
			p.b = append(p.b, ',')
		}
		p.num(v)
	}
}

// arc appends a circular arc of radius r to the relative point (dx, dy).
func (p *svgPath) arc(r, dx, dy float64, large bool) {
	p.cmd('a', r, r)
	if large {
		p.b = append(p.b, " 0 1,0 "...)
	} else {
		p.b = append(p.b, " 0 0,1 "...)
	}
	p.num(dx)
	p.b = append(p.b, ',')
	p.num(dy)
}

// circle appends a circle as two arcs.
func (p *svgPath) circle(cx, cy, r float64) {
	p.cmd('M', cx-r, cy)
	p.arc(r, 2*r, 0, true)
	p.arc(r, -2*r, 0, true)
	p.b = append(p.b, 'z')
}

// roundedSquare appends a clockwise square at (x, y) of side w with the
// given corner radii (top-left, top-right, bottom-right, bottom-left).
func (p *svgPath) roundedSquare(x, y, w, tl, tr, br, bl float64) {
	p.cmd('M', x+tl, y)
	p.cmd('H', x+w-tr)
	if tr > 0 {
		p.arc(tr, tr, tr, false)
	}
	p.cmd('V', y+w-br)
	if br > 0 {
		p.arc(br, -br, br, false)
	}
	p.cmd('H', x+bl)
	if bl > 0 {
		p.arc(bl, -bl, -bl, false)
	}
	p.cmd('V', y+tl)
	if tl > 0 {
		p.arc(tl, tl, -tl, false)
	}
	p.b = append(p.b, 'z')
}

// box appends b, scaled by s and offset by (ox, oy).
func (p *svgPath) box(b roundedBox, ox, oy, s float64) {
	r := b.r * s
	p.roundedSquare(ox+b.x*s, oy+b.y*s, b.w*s, r, r, r, r)
}
