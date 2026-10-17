package render

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/piglig/go-qr/v2/internal/errs"
)

type Gradient struct {
	From, To color.Color
	Angle    float64 // degrees
}

func (g *Gradient) Validate() error {
	switch {
	case g.From == nil || g.To == nil:
		return fmt.Errorf("%w: nil gradient color", errs.ErrInvalidArgument)
	case math.IsNaN(g.Angle) || math.IsInf(g.Angle, 0):
		return fmt.Errorf("%w: gradient angle %v", errs.ErrInvalidArgument, g.Angle)
	}
	return nil
}

// axis returns the gradient's start point and the vector to its end over a
// side×side image: the line through the center along the angle, long enough
// that the gradient spans every corner.
func (g *Gradient) axis(side float64) (x0, y0, dx, dy float64) {
	sin, cos := math.Sincos(g.Angle * math.Pi / 180)
	half := (math.Abs(cos) + math.Abs(sin)) * side / 2
	return side/2 - cos*half, side/2 - sin*half, 2 * cos * half, 2 * sin * half
}

// field returns the gradient over a side×side image, with its axis computed
// once rather than per pixel.
func (g *Gradient) field(side int) gradientField {
	x0, y0, dx, dy := g.axis(float64(side))
	return gradientField{x0, y0, dx, dy, dx*dx + dy*dy, rgba(g.From), rgba(g.To)}
}

// gradientField is a gradient laid over an image.
type gradientField struct {
	x0, y0, dx, dy, norm float64
	from, to             color.RGBA
}

// at returns the gradient color at pixel (x, y).
func (f *gradientField) at(x, y int) color.RGBA {
	t := ((float64(x)+0.5-f.x0)*f.dx + (float64(y)+0.5-f.y0)*f.dy) / f.norm
	t = math.Max(0, math.Min(1, t))
	return lerpRGBA(f.from, f.to, uint32(t*255+0.5))
}

// ID returns an element ID that differs between gradients, so several SVGs
// inlined in one HTML page do not resolve each other's gradients.
func (g *Gradient) ID() string {
	h := fnv.New32a()
	fmt.Fprintf(h, "%v|%v|%v", rgba(g.From), rgba(g.To), g.Angle)
	return fmt.Sprintf("qr-gradient-%08x", h.Sum32())
}

// writeSVGDef writes the <linearGradient> element for a side×side image.
func (g *Gradient) writeSVGDef(sb *strings.Builder, side int) {
	x0, y0, dx, dy := g.axis(float64(side))
	var p svgPath
	sb.WriteString("\t<defs><linearGradient id=\"" + g.ID() + "\" gradientUnits=\"userSpaceOnUse\" x1=\"")
	for i, v := range []float64{x0, y0, x0 + dx, y0 + dy} {
		p.b = p.b[:0]
		p.num(v)
		sb.Write(p.b)
		if i < 3 {
			sb.WriteString([]string{"\" y1=\"", "\" x2=\"", "\" y2=\""}[i])
		}
	}
	sb.WriteString("\"><stop offset=\"0\" stop-color=\"" + ColorToSVG(g.From) + "\"/><stop offset=\"1\" stop-color=\"" +
		ColorToSVG(g.To) + "\"/></linearGradient></defs>\n")
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
func (c *Config) multicolor() bool {
	return c.Gradient != nil || c.FinderColorSet
}

// DarkColors returns every color dark modules are painted with.
func (c *Config) DarkColors() []color.Color {
	var out []color.Color
	if c.Gradient != nil {
		out = append(out, c.Gradient.From, c.Gradient.To)
	} else {
		out = append(out, c.FG)
	}
	if c.FinderColorSet {
		out = append(out, c.FinderRing, c.FinderCenter)
	}
	return out
}

// svgFills returns the fill attribute values for data modules, finder rings
// and finder centers.
func (c *Config) svgFills() (data, ring, center string) {
	data = ColorToSVG(c.FG)
	if c.Gradient != nil {
		data = "url(#" + c.Gradient.ID() + ")"
	}
	ring, center = data, data
	if c.FinderColorSet {
		ring, center = ColorToSVG(c.FinderRing), ColorToSVG(c.FinderCenter)
	}
	return data, ring, center
}

// paintColored renders code with the configured shapes, finder colors and
// gradient into a new RGBA image. Module cells and finder parts do not
// overlap, so each pixel is the background blended toward its own color by
// its coverage.
func paintColored(code *Symbol, c *Config, side int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	bg := rgba(c.BG)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg.R, bg.G, bg.B, bg.A
	}
	fg := rgba(c.FG)
	var grad gradientField
	if c.Gradient != nil {
		grad = c.Gradient.field(side)
	}
	dataColor := func(x, y int) color.RGBA {
		if c.Gradient != nil {
			return grad.at(x, y)
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

	s, size, qz := c.Scale, code.Size(), c.QuietZone
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !code.Module(x, y) {
				continue
			}
			if _, _, ok := FinderOrigin(x, y, size); ok {
				continue
			}
			corners := 0
			if c.ModuleShape == ModuleRounded {
				corners = roundedCorners(code, x, y)
			}
			paint(moduleTile(c.ModuleShape, corners, s), s, (x+qz)*s, (y+qz)*s, dataColor)
		}
	}

	ringColor, centerColor := dataColor, dataColor
	if c.FinderColorSet {
		ring, center := rgba(c.FinderRing), rgba(c.FinderCenter)
		ringColor = func(int, int) color.RGBA { return ring }
		centerColor = func(int, int) color.RGBA { return center }
	}
	ringTile, centerTile := finderPartTiles(c.FinderShape, s)
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
