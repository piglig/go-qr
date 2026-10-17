package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"

	"github.com/piglig/go-qr/v2/internal/render"
)

// RenderOption configures the rendering methods of Code.
type RenderOption func(*renderConfig)

// renderConfig is the configuration the options build; package render
// draws from it.
type renderConfig = render.Config

const (
	defaultScale     = 10
	defaultQuietZone = 4
)

func newRenderConfig(opts []RenderOption) (renderConfig, error) {
	c := renderConfig{Scale: defaultScale, QuietZone: defaultQuietZone, FG: color.Black, BG: color.White}
	for _, o := range opts {
		o(&c)
	}
	switch {
	case c.Scale < 1:
		return c, fmt.Errorf("%w: scale %d must be at least 1", ErrInvalidArgument, c.Scale)
	case c.QuietZone < 0:
		return c, fmt.Errorf("%w: quiet zone %d must not be negative", ErrInvalidArgument, c.QuietZone)
	case c.FG == nil || c.BG == nil:
		return c, fmt.Errorf("%w: nil color", ErrInvalidArgument)
	case !ModuleShape(c.ModuleShape).valid():
		return c, fmt.Errorf("%w: unknown module shape %d", ErrInvalidArgument, c.ModuleShape)
	case !FinderShape(c.FinderShape).valid():
		return c, fmt.Errorf("%w: unknown finder shape %d", ErrInvalidArgument, c.FinderShape)
	case c.FinderColorSet && (c.FinderRing == nil || c.FinderCenter == nil):
		return c, fmt.Errorf("%w: nil finder color", ErrInvalidArgument)
	}
	if c.Gradient != nil {
		if err := c.Gradient.Validate(); err != nil {
			return c, err
		}
	}
	if c.Logo != nil {
		if err := validateLogoOptions(c.Logo); err != nil {
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
	return func(c *renderConfig) { c.Scale = n }
}

// WithQuietZone sets the width of the light margin around the symbol, in
// modules. ISO/IEC 18004 requires 4 (the default); many readers cope with 2.
func WithQuietZone(modules int) RenderOption {
	return func(c *renderConfig) { c.QuietZone = modules }
}

// WithForeground sets the color of dark modules. The default is black.
// Readers need strong contrast between foreground and background.
func WithForeground(c color.Color) RenderOption {
	return func(rc *renderConfig) { rc.FG = c }
}

// WithBackground sets the color of light modules and the quiet zone. The
// default is white. A fully transparent background is allowed; SVG output
// then omits the background rectangle.
func WithBackground(c color.Color) RenderOption {
	return func(rc *renderConfig) { rc.BG = c }
}

// WithSVGXMLHeader adds an XML declaration and DOCTYPE to SVG output, for
// tools that require a standalone document.
func WithSVGXMLHeader() RenderOption {
	return func(c *renderConfig) { c.XMLHeader = true }
}

// checkLogo reports whether the logo of c, if any, fits the error
// correction budget of code.
func checkLogo(c *renderConfig, code *Code) error {
	if c.Logo == nil {
		return nil
	}
	return validateLogo(c.Logo, code)
}

// symbol returns the modules of c for package render.
func (c *Code) symbol() render.Symbol { return render.Symbol{N: c.size, Modules: c.modules} }

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
	return func(c *renderConfig) { c.ModuleShape = render.ModuleShape(s) }
}

// WithFinderShape sets the shape of the three finder patterns. Every shape
// keeps the 1:1:3:1:1 proportions that readers look for along the rows and
// columns through the center. Text output ignores it.
func WithFinderShape(s FinderShape) RenderOption {
	return func(c *renderConfig) { c.FinderShape = render.FinderShape(s) }
}

// WithFinderColor sets the colors of the three finder patterns: the outer
// ring and the center. By default they use the foreground, or the gradient.
// Readers need them to contrast with the background as much as the data;
// Code.Verify checks it.
func WithFinderColor(ring, center color.Color) RenderOption {
	return func(c *renderConfig) { c.FinderRing, c.FinderCenter, c.FinderColorSet = ring, center, true }
}

// WithGradient paints the dark modules with a linear gradient from one
// color to another, replacing the foreground. angle is in degrees: 0 runs
// left to right, 90 top to bottom. Finder patterns follow the gradient
// unless WithFinderColor is given. Keep both colors dark against the
// background; Code.Verify checks the contrast of each end.
func WithGradient(from, to color.Color, angle float64) RenderOption {
	return func(c *renderConfig) { c.Gradient = &render.Gradient{From: from, To: to, Angle: angle} }
}

// Image renders the symbol, quiet zone and logo into a new RGBA image. Use it
// to compose the code with other graphics before encoding.
func (c *Code) Image(opts ...RenderOption) (*image.RGBA, error) {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return nil, err
	}
	return c.renderRGBA(&cfg)
}

// renderRGBA checks the image size and the logo, and renders.
func (c *Code) renderRGBA(cfg *renderConfig) (*image.RGBA, error) {
	sym := c.symbol()
	if _, err := cfg.RasterSide(&sym); err != nil {
		return nil, err
	}
	if err := checkLogo(cfg, c); err != nil {
		return nil, err
	}
	return render.RGBA(&sym, cfg)
}

// WritePNG writes the symbol as a PNG image to w.
func (c *Code) WritePNG(w io.Writer, opts ...RenderOption) error {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return err
	}
	sym := c.symbol()
	if cfg.Logo != nil {
		if _, err := cfg.RasterSide(&sym); err != nil {
			return err
		}
		if err := checkLogo(&cfg, c); err != nil {
			return err
		}
	}
	return render.WritePNG(w, &sym, &cfg)
}

// PNG returns the symbol encoded as a PNG image.
func (c *Code) PNG(opts ...RenderOption) ([]byte, error) {
	var buf bytes.Buffer
	if err := c.WritePNG(&buf, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteSVG writes the symbol as an SVG document to w. The dark modules are
// drawn as a single path whose outlines trace each connected region, with
// fill-rule="evenodd" punching out the holes, which keeps the output compact.
func (c *Code) WriteSVG(w io.Writer, opts ...RenderOption) error {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return err
	}
	sym := c.symbol()
	if _, err := cfg.SidePixels(&sym); err != nil {
		return err
	}
	if err := checkLogo(&cfg, c); err != nil {
		return err
	}
	return render.WriteSVG(w, &sym, &cfg)
}

// SVG returns the symbol encoded as an SVG document.
func (c *Code) SVG(opts ...RenderOption) ([]byte, error) {
	var buf bytes.Buffer
	if err := c.WriteSVG(&buf, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteText writes the symbol as Unicode text, two module rows per line,
// using the block characters █ ▀ ▄ for dark modules and spaces for light
// ones. It renders correctly as dark-on-light, for example in a light
// terminal or a document; on a dark terminal use WithForeground and
// WithBackground with the PNG or SVG output instead, or invert the colors in
// the terminal. Only WithQuietZone applies.
func (c *Code) WriteText(w io.Writer, opts ...RenderOption) error {
	cfg, err := newRenderConfig(opts)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, c.text(cfg.QuietZone)); err != nil {
		return fmt.Errorf("qr: write text: %w", err)
	}
	return nil
}

// String returns the symbol as Unicode block text with the default quiet
// zone, as written by WriteText.
func (c *Code) String() string {
	return c.text(defaultQuietZone)
}

func (c *Code) text(quietZone int) string {
	lo, hi := -quietZone, c.size+quietZone
	var sb strings.Builder
	sb.Grow(((hi - lo + 1) / 2) * ((hi-lo)*3 + 1))
	for y := lo; y < hi; y += 2 {
		for x := lo; x < hi; x++ {
			top, bottom := c.Module(x, y), c.Module(x, y+1)
			switch {
			case top && bottom:
				sb.WriteRune('█')
			case top:
				sb.WriteRune('▀')
			case bottom:
				sb.WriteRune('▄')
			default:
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
