package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/piglig/go-qr/v2/internal/errs"
)

type Config struct {
	Scale     int
	QuietZone int
	FG, BG    color.Color
	Logo      *Logo
	XMLHeader bool

	ModuleShape ModuleShape
	FinderShape FinderShape

	FinderRing, FinderCenter color.Color
	FinderColorSet           bool
	Gradient                 *Gradient
}

// SidePixels returns the rendered image side, quiet zone included, or an
// error if it would overflow.
func (c *Config) SidePixels(code *Symbol) (int, error) {
	modules := int64(code.Size()) + 2*int64(c.QuietZone)
	side := modules * int64(c.Scale)
	if side > math.MaxInt32 {
		return 0, fmt.Errorf("%w: image side %d exceeds the maximum", errs.ErrInvalidArgument, side)
	}
	return int(side), nil
}

// maxRasterSide bounds the side of PNG and Image output, in pixels: an
// RGBA image of this side takes 1 GiB. Without a bound, a scale taken from
// a request could make a server allocate any amount of memory.
const maxRasterSide = 16384

// RasterSide is sidePixels for output whose pixels are allocated.
func (c *Config) RasterSide(code *Symbol) (int, error) {
	side, err := c.SidePixels(code)
	if err != nil {
		return 0, err
	}
	if side > maxRasterSide {
		return 0, fmt.Errorf("%w: image side %d pixels exceeds the maximum of %d", errs.ErrInvalidArgument, side, maxRasterSide)
	}
	return side, nil
}

// rgba converts a color to 8-bit alpha-premultiplied RGBA once, so the
// painters can write pixel bytes directly.
func rgba(c color.Color) color.RGBA {
	return color.RGBAModel.Convert(c).(color.RGBA)
}

// moduleRow fills row with one value per pixel for module row y of code,
// quiet zone included: dark pixels get fg and light pixels get bg.
func moduleRow[T any](row []T, code *Symbol, c *Config, y int, fg, bg T) {
	px := 0
	for mx := -c.QuietZone; mx < code.Size()+c.QuietZone; mx++ {
		v := bg
		if code.Module(mx, y) {
			v = fg
		}
		for i := 0; i < c.Scale; i++ {
			row[px] = v
			px++
		}
	}
}

// paintRGBA renders code into a new RGBA image without any logo.
func paintRGBA(code *Symbol, c *Config, side int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	fg, bg := rgba(c.FG), rgba(c.BG)
	row := make([]color.RGBA, side)
	for my := -c.QuietZone; my < code.Size()+c.QuietZone; my++ {
		moduleRow(row, code, c, my, fg, bg)
		y0 := (my + c.QuietZone) * c.Scale
		line := img.Pix[y0*img.Stride : y0*img.Stride+side*4]
		for x, p := range row {
			line[4*x], line[4*x+1], line[4*x+2], line[4*x+3] = p.R, p.G, p.B, p.A
		}
		for dy := 1; dy < c.Scale; dy++ {
			copy(img.Pix[(y0+dy)*img.Stride:], line)
		}
	}
	return img
}
