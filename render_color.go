package qr

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"strings"
)

// WithFinderColor sets the colors of the three finder patterns: the outer
// ring and the center. By default they use the foreground, or the gradient.
// Readers need them to contrast with the background as much as the data;
// Code.Verify checks it.
func WithFinderColor(ring, center color.Color) RenderOption {
	return func(c *renderConfig) { c.finderRing, c.finderCenter, c.finderColorSet = ring, center, true }
}

// WithGradient paints the dark modules with a linear gradient from one
// color to another, replacing the foreground. angle is in degrees: 0 runs
// left to right, 90 top to bottom. Finder patterns follow the gradient
// unless WithFinderColor is given. Keep both colors dark against the
// background; Code.Verify checks the contrast of each end.
func WithGradient(from, to color.Color, angle float64) RenderOption {
	return func(c *renderConfig) { c.gradient = &gradientConfig{from: from, to: to, angle: angle} }
}

type gradientConfig struct {
	from, to color.Color
	angle    float64 // degrees
}

func (g *gradientConfig) validate() error {
	switch {
	case g.from == nil || g.to == nil:
		return fmt.Errorf("%w: nil gradient color", ErrInvalidArgument)
	case math.IsNaN(g.angle) || math.IsInf(g.angle, 0):
		return fmt.Errorf("%w: gradient angle %v", ErrInvalidArgument, g.angle)
	}
	return nil
}

// axis returns the gradient's start point and the vector to its end over a
// side×side image: the line through the center along the angle, long enough
// that the gradient spans every corner.
func (g *gradientConfig) axis(side float64) (x0, y0, dx, dy float64) {
	sin, cos := math.Sincos(g.angle * math.Pi / 180)
	half := (math.Abs(cos) + math.Abs(sin)) * side / 2
	return side/2 - cos*half, side/2 - sin*half, 2 * cos * half, 2 * sin * half
}

// at returns the gradient color at pixel (x, y) of a side×side image.
func (g *gradientConfig) at(x, y, side int, from, to color.RGBA) color.RGBA {
	x0, y0, dx, dy := g.axis(float64(side))
	t := ((float64(x)+0.5-x0)*dx + (float64(y)+0.5-y0)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return lerpRGBA(from, to, uint32(t*255+0.5))
}

// id returns an element id that differs between gradients, so several SVGs
// inlined in one HTML page do not resolve each other's gradients.
func (g *gradientConfig) id() string {
	h := fnv.New32a()
	fmt.Fprintf(h, "%v|%v|%v", rgba(g.from), rgba(g.to), g.angle)
	return fmt.Sprintf("qr-gradient-%08x", h.Sum32())
}

// writeSVGDef writes the <linearGradient> element for a side×side image.
func (g *gradientConfig) writeSVGDef(sb *strings.Builder, side int) {
	x0, y0, dx, dy := g.axis(float64(side))
	var p svgPath
	sb.WriteString("\t<defs><linearGradient id=\"" + g.id() + "\" gradientUnits=\"userSpaceOnUse\" x1=\"")
	for i, v := range []float64{x0, y0, x0 + dx, y0 + dy} {
		p.b = p.b[:0]
		p.num(v)
		sb.Write(p.b)
		if i < 3 {
			sb.WriteString([]string{"\" y1=\"", "\" x2=\"", "\" y2=\""}[i])
		}
	}
	sb.WriteString("\"><stop offset=\"0\" stop-color=\"" + colorToSVG(g.from) + "\"/><stop offset=\"1\" stop-color=\"" +
		colorToSVG(g.to) + "\"/></linearGradient></defs>\n")
}

// lerpRGBA blends a toward b by t/255.
func lerpRGBA(a, b color.RGBA, t uint32) color.RGBA {
	it := 255 - t
	return color.RGBA{
		R: uint8((uint32(a.R)*it + uint32(b.R)*t + 127) / 255),
		G: uint8((uint32(a.G)*it + uint32(b.G)*t + 127) / 255),
		B: uint8((uint32(a.B)*it + uint32(b.B)*t + 127) / 255),
		A: uint8((uint32(a.A)*it + uint32(b.A)*t + 127) / 255),
	}
}

// multicolor reports whether dark modules use more than one color.
func (c *renderConfig) multicolor() bool {
	return c.gradient != nil || c.finderColorSet
}

// darkColors returns every color dark modules are painted with.
func (c *renderConfig) darkColors() []color.Color {
	var out []color.Color
	if c.gradient != nil {
		out = append(out, c.gradient.from, c.gradient.to)
	} else {
		out = append(out, c.fg)
	}
	if c.finderColorSet {
		out = append(out, c.finderRing, c.finderCenter)
	}
	return out
}

// svgFills returns the fill attribute values for data modules, finder rings
// and finder centers.
func (c *renderConfig) svgFills() (data, ring, center string) {
	data = colorToSVG(c.fg)
	if c.gradient != nil {
		data = "url(#" + c.gradient.id() + ")"
	}
	ring, center = data, data
	if c.finderColorSet {
		ring, center = colorToSVG(c.finderRing), colorToSVG(c.finderCenter)
	}
	return data, ring, center
}

// paintColored renders code with the configured shapes, finder colors and
// gradient into a new RGBA image. Module cells and finder parts do not
// overlap, so each pixel is the background blended toward its own color by
// its coverage.
func paintColored(code *Code, c *renderConfig, side int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	bg := rgba(c.bg)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg.R, bg.G, bg.B, bg.A
	}
	fg := rgba(c.fg)
	var from, to color.RGBA
	if c.gradient != nil {
		from, to = rgba(c.gradient.from), rgba(c.gradient.to)
	}
	dataColor := func(x, y int) color.RGBA {
		if c.gradient != nil {
			return c.gradient.at(x, y, side, from, to)
		}
		return fg
	}
	paint := func(tile []uint8, px, x0, y0 int, col func(x, y int) color.RGBA) {
		for ty := 0; ty < px; ty++ {
			row := img.Pix[(y0+ty)*img.Stride+x0*4:]
			for tx, a := range tile[ty*px : (ty+1)*px] {
				if a == 0 {
					continue
				}
				p := lerpRGBA(bg, col(x0+tx, y0+ty), uint32(a))
				row[tx*4], row[tx*4+1], row[tx*4+2], row[tx*4+3] = p.R, p.G, p.B, p.A
			}
		}
	}

	s, size, qz := c.scale, code.Size(), c.quietZone
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
			paint(moduleTile(c.moduleShape, corners, s), s, (x+qz)*s, (y+qz)*s, dataColor)
		}
	}

	ringColor, centerColor := dataColor, dataColor
	if c.finderColorSet {
		ring, center := rgba(c.finderRing), rgba(c.finderCenter)
		ringColor = func(int, int) color.RGBA { return ring }
		centerColor = func(int, int) color.RGBA { return center }
	}
	ringTile, centerTile := finderPartTiles(c.finderShape, s)
	for _, o := range [3][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
		paint(ringTile, 7*s, (o[0]+qz)*s, (o[1]+qz)*s, ringColor)
		paint(centerTile, 7*s, (o[0]+qz)*s, (o[1]+qz)*s, centerColor)
	}
	return img
}

// finderPartTiles returns the coverage of a finder's ring and center
// separately, for coloring them independently.
func finderPartTiles(shape FinderShape, scale int) (ring, center []uint8) {
	g := finderShapeGeometry(shape)
	ring = cachedTile(tileKey{2, int(shape), 0, scale}, 7*scale, 7, func(u, v float64) bool {
		return g.outer.contains(u, v) && !g.hole.contains(u, v)
	})
	center = cachedTile(tileKey{3, int(shape), 0, scale}, 7*scale, 7, g.center.contains)
	return ring, center
}
