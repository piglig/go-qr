package qr

import (
	"fmt"
	"io"
	"strings"
)

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
	if _, err := io.WriteString(w, c.text(cfg.quietZone)); err != nil {
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
