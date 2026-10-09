package qr

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// RenderOption configures the rendering methods of Code.
type RenderOption func(*renderConfig)

type renderConfig struct {
	scale     int
	quietZone int
	fg, bg    color.Color
	logo      *logoConfig
	xmlHeader bool

	moduleShape ModuleShape
	finderShape FinderShape

	finderRing, finderCenter color.Color
	finderColorSet           bool
	gradient                 *gradientConfig
}

const (
	defaultScale     = 10
	defaultQuietZone = 4
)

func newRenderConfig(opts []RenderOption) (renderConfig, error) {
	c := renderConfig{scale: defaultScale, quietZone: defaultQuietZone, fg: color.Black, bg: color.White}
	for _, o := range opts {
		o(&c)
	}
	switch {
	case c.scale < 1:
		return c, fmt.Errorf("%w: scale %d must be at least 1", ErrInvalidArgument, c.scale)
	case c.quietZone < 0:
		return c, fmt.Errorf("%w: quiet zone %d must not be negative", ErrInvalidArgument, c.quietZone)
	case c.fg == nil || c.bg == nil:
		return c, fmt.Errorf("%w: nil color", ErrInvalidArgument)
	case !c.moduleShape.valid():
		return c, fmt.Errorf("%w: unknown module shape %d", ErrInvalidArgument, c.moduleShape)
	case !c.finderShape.valid():
		return c, fmt.Errorf("%w: unknown finder shape %d", ErrInvalidArgument, c.finderShape)
	case c.finderColorSet && (c.finderRing == nil || c.finderCenter == nil):
		return c, fmt.Errorf("%w: nil finder color", ErrInvalidArgument)
	}
	if c.gradient != nil {
		if err := c.gradient.validate(); err != nil {
			return c, err
		}
	}
	if c.logo != nil {
		if err := c.logo.validateOptions(); err != nil {
			return c, err
		}
	}
	return c, nil
}

// WithScale sets the side of one module: pixels for PNG and Image, user units
// for SVG. The default is 10. Text output ignores it. PNG and Image fail
// with ErrInvalidArgument when the image, quiet zone included, would be
// more than 16384 pixels on a side.
func WithScale(n int) RenderOption {
	return func(c *renderConfig) { c.scale = n }
}

// WithQuietZone sets the width of the light margin around the symbol, in
// modules. ISO/IEC 18004 requires 4 (the default); many readers cope with 2.
func WithQuietZone(modules int) RenderOption {
	return func(c *renderConfig) { c.quietZone = modules }
}

// WithForeground sets the color of dark modules. The default is black.
// Readers need strong contrast between foreground and background.
func WithForeground(c color.Color) RenderOption {
	return func(rc *renderConfig) { rc.fg = c }
}

// WithBackground sets the color of light modules and the quiet zone. The
// default is white. A fully transparent background is allowed; SVG output
// then omits the background rectangle.
func WithBackground(c color.Color) RenderOption {
	return func(rc *renderConfig) { rc.bg = c }
}

// WithSVGXMLHeader adds an XML declaration and DOCTYPE to SVG output, for
// tools that require a standalone document.
func WithSVGXMLHeader() RenderOption {
	return func(c *renderConfig) { c.xmlHeader = true }
}

// sidePixels returns the rendered image side, quiet zone included, or an
// error if it would overflow.
func (c *renderConfig) sidePixels(code *Code) (int, error) {
	modules := int64(code.Size()) + 2*int64(c.quietZone)
	side := modules * int64(c.scale)
	if side > math.MaxInt32 {
		return 0, fmt.Errorf("%w: image side %d exceeds the maximum", ErrInvalidArgument, side)
	}
	return int(side), nil
}

// maxRasterSide bounds the side of PNG and Image output, in pixels: an
// RGBA image of this side takes 1 GiB. Without a bound, a scale taken from
// a request could make a server allocate any amount of memory.
const maxRasterSide = 16384

// rasterSide is sidePixels for output whose pixels are allocated.
func (c *renderConfig) rasterSide(code *Code) (int, error) {
	side, err := c.sidePixels(code)
	if err != nil {
		return 0, err
	}
	if side > maxRasterSide {
		return 0, fmt.Errorf("%w: image side %d pixels exceeds the maximum of %d", ErrInvalidArgument, side, maxRasterSide)
	}
	return side, nil
}

// checkLogo reports whether the configured logo, if any, fits the error
// correction budget of code.
func (c *renderConfig) checkLogo(code *Code) error {
	if c.logo == nil {
		return nil
	}
	return c.logo.validate(code)
}

// rgba converts a color to 8-bit alpha-premultiplied RGBA once, so the
// painters can write pixel bytes directly.
func rgba(c color.Color) color.RGBA {
	return color.RGBAModel.Convert(c).(color.RGBA)
}

// moduleRow fills row with one value per pixel for module row y of code,
// quiet zone included: dark pixels get fg and light pixels get bg.
func moduleRow[T any](row []T, code *Code, c *renderConfig, y int, fg, bg T) {
	px := 0
	for mx := -c.quietZone; mx < code.Size()+c.quietZone; mx++ {
		v := bg
		if code.Module(mx, y) {
			v = fg
		}
		for i := 0; i < c.scale; i++ {
			row[px] = v
			px++
		}
	}
}

// paintRGBA renders code into a new RGBA image without any logo.
func paintRGBA(code *Code, c *renderConfig, side int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	fg, bg := rgba(c.fg), rgba(c.bg)
	row := make([]color.RGBA, side)
	for my := -c.quietZone; my < code.Size()+c.quietZone; my++ {
		moduleRow(row, code, c, my, fg, bg)
		y0 := (my + c.quietZone) * c.scale
		line := img.Pix[y0*img.Stride : y0*img.Stride+side*4]
		for x, p := range row {
			line[4*x], line[4*x+1], line[4*x+2], line[4*x+3] = p.R, p.G, p.B, p.A
		}
		for dy := 1; dy < c.scale; dy++ {
			copy(img.Pix[(y0+dy)*img.Stride:], line)
		}
	}
	return img
}

// paintPaletted renders code into a two-color paletted image, which the PNG
// encoder stores at one bit per pixel.
func paintPaletted(code *Code, c *renderConfig, side int) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, side, side), color.Palette{c.bg, c.fg})
	for my := -c.quietZone; my < code.Size()+c.quietZone; my++ {
		y0 := (my + c.quietZone) * c.scale
		line := img.Pix[y0*img.Stride : y0*img.Stride+side]
		moduleRow(line, code, c, my, 1, 0)
		for dy := 1; dy < c.scale; dy++ {
			copy(img.Pix[(y0+dy)*img.Stride:], line)
		}
	}
	return img
}
